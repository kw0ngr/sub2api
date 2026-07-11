#!/usr/bin/env python3
"""Convert bulk Grok account lines into Sub2API OAuth refresh tokens.

Input line format (pipe or ---- separated):
  email|password|sso_token|optional_timestamp

Core capability
---------------
Pure HTTP SSO -> OAuth RT (no browser required):

  1. Cookie: sso=<token>; sso-rw=<token>  (same shape as grok2api)
  2. GET  accounts.x.ai/sign-in?redirect=grok-com&email=true   # warm session
  3. GET  auth.x.ai/oauth2/authorize?...PKCE...               # consent HTML
  4. POST auth.x.ai/oauth2/authorize  fields + decision=allow # -> ?code=
  5. POST auth.x.ai/oauth2/token      authorization_code      # -> refresh_token

Then optionally import RTs into Sub2API admin:
  POST /api/v1/admin/grok/oauth/import-refresh-tokens
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import http.cookiejar
import json
import os
import re
import secrets
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

DEFAULT_CLIENT_ID = "b1a00492-073a-47ea-816f-4c329264a828"
DEFAULT_SCOPE = "openid profile email offline_access grok-cli:access api:access"
DEFAULT_AUTHORIZE_URL = "https://auth.x.ai/oauth2/authorize"
DEFAULT_TOKEN_URL = "https://auth.x.ai/oauth2/token"
DEFAULT_REDIRECT_URI = "http://127.0.0.1:56121/callback"
DEFAULT_UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/136.0.0.0 Safari/537.36"
)


@dataclass
class AccountLine:
    line_no: int
    email: str
    password: str
    sso: str
    timestamp: str = ""
    raw: str = ""


@dataclass
class AccountResult:
    line_no: int
    email: str
    sso_preview: str
    sso_valid: bool | None = None
    sso_probe_detail: str = ""
    refresh_token: str = ""
    access_token: str = ""
    oauth_status: str = "skipped"
    oauth_detail: str = ""
    rt_probe_status: str = "skipped"
    rt_probe_detail: str = ""
    rt_usable: bool | None = None
    import_status: str = "skipped"
    import_detail: str = ""
    account_id: int | None = None
    error: str = ""
    extras: dict[str, Any] = field(default_factory=dict)


def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def preview_token(token: str, head: int = 8, tail: int = 4) -> str:
    token = (token or "").strip()
    if len(token) <= head + tail + 1:
        return "***"
    return f"{token[:head]}…{token[-tail:]}"


def decode_jwt_part(part: str) -> dict[str, Any] | None:
    raw = part.strip()
    if not raw:
        return None
    pad = "=" * ((4 - len(raw) % 4) % 4)
    try:
        data = base64.urlsafe_b64decode(raw + pad)
        obj = json.loads(data.decode("utf-8"))
        return obj if isinstance(obj, dict) else None
    except Exception:
        return None


def strip_sso_prefix(token: str) -> str:
    tok = token.strip()
    lower = tok.lower()
    for prefix in ("sso=", "sso:", "cookie:sso=", "cookie: sso="):
        if lower.startswith(prefix):
            return tok[len(prefix) :].strip()
    m = re.search(r"(?:^|[;\s])sso=([^;]+)", tok, flags=re.I)
    if m:
        return m.group(1).strip()
    return tok


def parse_account_lines(text: str) -> list[AccountLine]:
    accounts: list[AccountLine] = []
    seen: set[str] = set()
    for idx, raw in enumerate(text.splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        # support both | and ---- separators used by card sellers
        if "----" in line and "|" not in line:
            parts = [p.strip() for p in line.split("----")]
        else:
            parts = [p.strip() for p in line.split("|")]
        if len(parts) < 3:
            raise ValueError(f"line {idx}: expected email|password|sso|..., got {line!r}")
        email, password, sso = parts[0], parts[1], strip_sso_prefix(parts[2])
        timestamp = parts[3] if len(parts) >= 4 else ""
        if not email or not sso:
            raise ValueError(f"line {idx}: email and sso are required")
        if sso in seen:
            continue
        seen.add(sso)
        accounts.append(
            AccountLine(
                line_no=idx,
                email=email,
                password=password,
                sso=sso,
                timestamp=timestamp,
                raw=line,
            )
        )
    return accounts


def http_json(
    method: str,
    url: str,
    *,
    headers: dict[str, str] | None = None,
    json_body: Any | None = None,
    form_body: dict[str, str] | None = None,
    timeout: float = 30.0,
) -> tuple[int, dict[str, str], bytes]:
    data: bytes | None = None
    req_headers = {"User-Agent": DEFAULT_UA, "Accept": "application/json, */*"}
    if headers:
        req_headers.update(headers)
    if json_body is not None:
        data = json.dumps(json_body).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    if form_body is not None:
        data = urllib.parse.urlencode(form_body).encode("utf-8")
        req_headers["Content-Type"] = "application/x-www-form-urlencoded"
    req = urllib.request.Request(url, data=data, method=method.upper(), headers=req_headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, {k: v for k, v in resp.headers.items()}, resp.read()
    except urllib.error.HTTPError as err:
        body = err.read() if err.fp is not None else b""
        return err.code, {k: v for k, v in err.headers.items()}, body


def probe_sso(sso: str, timeout: float = 20.0) -> tuple[bool | None, str]:
    last_err = ""
    for attempt in range(1, 4):
        try:
            status, _, body = http_json(
                "POST",
                "https://grok.com/rest/rate-limits",
                headers={
                    "Content-Type": "application/json",
                    "Origin": "https://grok.com",
                    "Referer": "https://grok.com/",
                    "Cookie": f"sso={sso}; sso-rw={sso}",
                },
                json_body={"modelName": "fast"},
                timeout=timeout,
            )
        except Exception as exc:
            last_err = str(exc)
            time.sleep(0.6 * attempt)
            continue
        text = body.decode("utf-8", errors="replace")
        if status == 200:
            try:
                obj = json.loads(text)
                return True, f"ok remaining={obj.get('remainingQueries')}/{obj.get('totalQueries')}"
            except Exception:
                return True, "ok"
        if status in (401, 403):
            return False, f"auth failed HTTP {status}: {text[:180]}"
        if status in (429, 502, 503, 520, 521, 522, 523, 524):
            last_err = f"transient HTTP {status}: {text[:180]}"
            time.sleep(0.6 * attempt)
            continue
        return False, f"unexpected HTTP {status}: {text[:180]}"
    return None, f"probe failed after retries: {last_err[:200]}"


def exchange_code_for_tokens(
    code: str,
    code_verifier: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    redirect_uri: str = DEFAULT_REDIRECT_URI,
    token_url: str = DEFAULT_TOKEN_URL,
    timeout: float = 30.0,
) -> dict[str, Any]:
    status, _, body = http_json(
        "POST",
        token_url,
        form_body={
            "grant_type": "authorization_code",
            "client_id": client_id,
            "code": code,
            "redirect_uri": redirect_uri,
            "code_verifier": code_verifier,
        },
        timeout=timeout,
    )
    text = body.decode("utf-8", errors="replace")
    if status >= 400:
        raise RuntimeError(f"token exchange failed HTTP {status}: {text[:300]}")
    obj = json.loads(text)
    if not isinstance(obj, dict):
        raise RuntimeError("token exchange returned non-object JSON")
    return obj



def refresh_oauth_token(
    refresh_token: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    token_url: str = DEFAULT_TOKEN_URL,
    timeout: float = 30.0,
) -> dict[str, Any]:
    status, _, body = http_json(
        "POST",
        token_url,
        form_body={
            "grant_type": "refresh_token",
            "client_id": client_id,
            "refresh_token": refresh_token,
        },
        timeout=timeout,
    )
    text = body.decode("utf-8", errors="replace")
    if status >= 400:
        raise RuntimeError(f"refresh failed HTTP {status}: {text[:300]}")
    obj = json.loads(text)
    if not isinstance(obj, dict) or not str(obj.get("access_token") or "").strip():
        raise RuntimeError(f"refresh response missing access_token: {text[:200]}")
    return obj


def probe_refresh_token_usability(
    refresh_token: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    timeout: float = 30.0,
) -> tuple[bool, str, dict[str, Any]]:
    """Probe whether an OAuth RT can become a usable api.x.ai access token.

    Levels:
      - refresh_ok: RT can mint access_token (typ=at+jwt expected)
      - models_ok: GET /v1/models accepts AT
      - chat_ok: POST /v1/chat/completions accepts AT

    Returns (usable_for_import, detail, meta).
    usable_for_import is True when refresh_ok (credential is real OAuth RT).
    chat_ok may still be false for web-only accounts without API entitlement.
    """
    meta: dict[str, Any] = {
        "refresh_ok": False,
        "models_ok": False,
        "chat_ok": False,
        "access_token_typ": "",
        "models_status": None,
        "chat_status": None,
        "chat_error": "",
    }
    try:
        token = refresh_oauth_token(refresh_token, client_id=client_id, timeout=timeout)
    except Exception as exc:
        return False, f"refresh failed: {exc}", meta

    at = str(token.get("access_token") or "").strip()
    meta["refresh_ok"] = True
    meta["expires_in"] = token.get("expires_in")
    # decode JWT header typ if possible
    try:
        hdr = decode_jwt_part(at.split(".")[0]) or {}
        meta["access_token_typ"] = str(hdr.get("typ") or "")
        meta["access_token_alg"] = str(hdr.get("alg") or "")
    except Exception:
        pass

    # models probe
    try:
        st, _, body = http_json(
            "GET",
            "https://api.x.ai/v1/models",
            headers={"Authorization": f"Bearer {at}", "Accept": "application/json"},
            timeout=timeout,
        )
        meta["models_status"] = st
        meta["models_ok"] = 200 <= st < 300
        if not meta["models_ok"]:
            meta["models_error"] = body.decode("utf-8", errors="replace")[:180]
    except Exception as exc:
        meta["models_error"] = str(exc)

    # chat probe (minimal)
    try:
        st, _, body = http_json(
            "POST",
            "https://api.x.ai/v1/chat/completions",
            headers={
                "Authorization": f"Bearer {at}",
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
            json_body={
                "model": "grok-3",
                "messages": [{"role": "user", "content": "ping"}],
                "max_tokens": 1,
            },
            timeout=timeout,
        )
        meta["chat_status"] = st
        # 200 = ok; 429 = authenticated but rate-limited; 400 may be model/param
        if 200 <= st < 300 or st == 429:
            meta["chat_ok"] = True
        else:
            msg = body.decode("utf-8", errors="replace")
            meta["chat_error"] = msg[:220]
            # model not found still means auth worked
            low = msg.lower()
            if st == 400 and ("model" in low or "invalid" in low) and "permission" not in low and "incorrect api key" not in low:
                meta["chat_ok"] = True
                meta["chat_note"] = "auth accepted; model/param issue"
    except Exception as exc:
        meta["chat_error"] = str(exc)

    usable = bool(meta["refresh_ok"])
    detail = (
        f"refresh_ok={meta['refresh_ok']} typ={meta.get('access_token_typ') or '?'} "
        f"models_ok={meta['models_ok']}({meta.get('models_status')}) "
        f"chat_ok={meta['chat_ok']}({meta.get('chat_status')})"
    )
    if meta.get("chat_error"):
        detail += f" chat_err={meta['chat_error'][:120]}"
    return usable, detail, meta


def sso_to_oauth_tokens_http(
    sso: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    redirect_uri: str = DEFAULT_REDIRECT_URI,
    authorize_url: str = DEFAULT_AUTHORIZE_URL,
    timeout: float = 30.0,
) -> tuple[str, str, dict[str, Any]]:
    """Pure HTTP SSO cookie -> refresh_token (+ access_token)."""
    sso = strip_sso_prefix(sso)
    if not sso:
        raise RuntimeError("empty sso")

    code_verifier = b64url(secrets.token_bytes(32))
    code_challenge = b64url(hashlib.sha256(code_verifier.encode("ascii")).digest())
    state = secrets.token_urlsafe(16)
    nonce = secrets.token_hex(8)

    jar = http.cookiejar.CookieJar()
    for name in ("sso", "sso-rw"):
        jar.set_cookie(
            http.cookiejar.Cookie(
                version=0,
                name=name,
                value=sso,
                port=None,
                port_specified=False,
                domain=".x.ai",
                domain_specified=True,
                domain_initial_dot=True,
                path="/",
                path_specified=True,
                secure=True,
                expires=None,
                discard=True,
                comment=None,
                comment_url=None,
                rest={},
                rfc2109=False,
            )
        )

    captured: dict[str, str | None] = {"url": None}

    class _StopLocalRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):  # type: ignore[no-untyped-def]
            if newurl.startswith("http://127.0.0.1") or newurl.startswith("http://localhost"):
                captured["url"] = newurl
                return None
            return super().redirect_request(req, fp, code, msg, headers, newurl)

    opener = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(jar),
        _StopLocalRedirect,
    )

    def _open(
        method: str,
        url: str,
        data: bytes | None = None,
        headers: dict[str, str] | None = None,
    ) -> tuple[int, dict[str, str], bytes, str]:
        h = {
            "User-Agent": DEFAULT_UA,
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        }
        if headers:
            h.update(headers)
        req = urllib.request.Request(url, data=data, method=method, headers=h)
        try:
            with opener.open(req, timeout=timeout) as resp:
                return resp.status, {k: v for k, v in resp.headers.items()}, resp.read(), resp.geturl()
        except urllib.error.HTTPError as err:
            body = err.read() if err.fp is not None else b""
            if captured["url"]:
                return err.code, {k: v for k, v in err.headers.items()}, body, captured["url"]
            loc = err.headers.get("Location") if err.headers is not None else None
            return (
                err.code,
                {k: v for k, v in err.headers.items()} if err.headers else {},
                body,
                loc or "",
            )
        except Exception:
            if captured["url"]:
                return 302, {}, b"", captured["url"]
            raise

    # 1) warm SSO session on accounts.x.ai
    _open("GET", "https://accounts.x.ai/sign-in?redirect=grok-com&email=true")

    # 2) PKCE authorize -> consent HTML
    params = {
        "response_type": "code",
        "client_id": client_id,
        "redirect_uri": redirect_uri,
        "scope": DEFAULT_SCOPE,
        "state": state,
        "code_challenge": code_challenge,
        "code_challenge_method": "S256",
        "plan": "generic",
        "referrer": "sub2api",
        "nonce": nonce,
    }
    auth_get_url = f"{authorize_url}?{urllib.parse.urlencode(params)}"
    status, _, body, final_url = _open(
        "GET",
        auth_get_url,
        headers={"Referer": "https://grok.com/"},
    )
    html = body.decode("utf-8", errors="replace")

    if captured["url"] and "code=" in captured["url"]:
        cb = captured["url"]
    else:
        fields: dict[str, str] = {}
        for m in re.finditer(r'<input[^>]+type="hidden"[^>]*>', html, flags=re.I):
            tag = m.group(0)
            name_m = re.search(r'name="([^"]+)"', tag)
            val_m = re.search(r'value="([^"]*)"', tag)
            if name_m:
                fields[name_m.group(1)] = val_m.group(1) if val_m else ""
        if not fields.get("client_id"):
            raise RuntimeError(
                f"consent form not found (status={status}, url={str(final_url)[:120]}, head={html[:160]!r})"
            )

        action_m = re.search(r'<form[^>]+action="([^"]+)"', html, flags=re.I)
        action = action_m.group(1) if action_m else "https://auth.x.ai/oauth2/authorize"
        payload = dict(fields)
        payload.setdefault("response_type", "code")
        payload.setdefault("plan", "generic")
        payload["decision"] = "allow"

        referer = final_url if isinstance(final_url, str) and final_url.startswith("http") else auth_get_url
        captured["url"] = None
        status2, headers2, body2, final2 = _open(
            "POST",
            action,
            data=urllib.parse.urlencode(payload).encode("utf-8"),
            headers={
                "Content-Type": "application/x-www-form-urlencoded",
                "Origin": "https://accounts.x.ai",
                "Referer": referer,
            },
        )
        cb = captured["url"] or (final2 if isinstance(final2, str) and "code=" in final2 else "")
        if not cb:
            loc = headers2.get("Location") or headers2.get("location") or ""
            if "code=" in loc:
                cb = loc
        if not cb:
            raise RuntimeError(
                f"consent POST did not return code (status={status2}, final={final2!r}, body={body2[:180]!r})"
            )

    values = urllib.parse.parse_qs(urllib.parse.urlparse(cb).query)
    code = (values.get("code") or [""])[0]
    got_state = (values.get("state") or [""])[0]
    if not code:
        raise RuntimeError(f"callback missing code: {cb[:200]}")
    if got_state and got_state != state:
        raise RuntimeError("oauth state mismatch")

    token = exchange_code_for_tokens(
        code,
        code_verifier,
        client_id=client_id,
        redirect_uri=redirect_uri,
        timeout=timeout,
    )
    rt = str(token.get("refresh_token") or "").strip()
    at = str(token.get("access_token") or "").strip()
    if not rt and not at:
        raise RuntimeError(f"token response missing credentials: {token}")
    return rt, at, token


def import_refresh_tokens(
    base_url: str,
    admin_token: str,
    refresh_tokens: list[str],
    *,
    name_prefix: str,
    group_ids: list[int],
    notes: str | None = None,
    import_concurrency: int = 5,
    confirm_mixed_channel_risk: bool = True,
    timeout: float = 120.0,
) -> dict[str, Any]:
    url = base_url.rstrip("/") + "/api/v1/admin/grok/oauth/import-refresh-tokens"
    payload: dict[str, Any] = {
        "refresh_tokens": refresh_tokens,
        "import_mode": "refresh_token",
        "name_prefix": name_prefix,
        "concurrency": 3,
        "import_concurrency": import_concurrency,
        "confirm_mixed_channel_risk": confirm_mixed_channel_risk,
    }
    if group_ids:
        payload["group_ids"] = group_ids
    if notes:
        payload["notes"] = notes
    status, _, body = http_json(
        "POST",
        url,
        headers={
            "Authorization": f"Bearer {admin_token}",
            "Content-Type": "application/json",
        },
        json_body=payload,
        timeout=timeout,
    )
    text = body.decode("utf-8", errors="replace")
    if status >= 400:
        raise RuntimeError(f"Sub2API import HTTP {status}: {text[:400]}")
    obj = json.loads(text)
    if isinstance(obj, dict) and isinstance(obj.get("data"), dict):
        return obj["data"]
    if isinstance(obj, dict):
        return obj
    raise RuntimeError(f"unexpected import response: {text[:200]}")



def _redact_import_summary(summary: dict[str, Any] | None) -> dict[str, Any] | None:
    if not summary:
        return summary
    out = dict(summary)
    results = []
    for item in out.get("results") or []:
        if not isinstance(item, dict):
            results.append(item)
            continue
        copy = {k: v for k, v in item.items() if k != "account"}
        acc = item.get("account")
        if isinstance(acc, dict):
            copy["account"] = {
                "id": acc.get("id"),
                "name": acc.get("name"),
                "platform": acc.get("platform"),
                "type": acc.get("type"),
                "status": acc.get("status"),
                "schedulable": acc.get("schedulable"),
            }
        results.append(copy)
    out["results"] = results
    return out


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    try:
        os.chmod(path, 0o600)
    except OSError:
        pass


def load_input_text(path: str | None) -> str:
    if path and path != "-":
        return Path(path).read_text(encoding="utf-8")
    if not sys.stdin.isatty():
        return sys.stdin.read()
    raise SystemExit("provide --input file or pipe account lines via stdin")


def build_arg_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="SSO dump -> pure HTTP OAuth RT -> optional Sub2API import",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    p.add_argument("--input", "-i", help="account dump file (default: stdin)")
    p.add_argument("--out-dir", default="tmp/grok_sso_import", help="report output directory")
    p.add_argument("--probe-sso", action=argparse.BooleanOptionalAction, default=True)
    p.add_argument("--probe-rt", action=argparse.BooleanOptionalAction, default=True,
                   help="after obtaining RT, probe refresh/models/chat usability (default: true)")
    p.add_argument("--import-only-chat-ok", action=argparse.BooleanOptionalAction, default=False,
                   help="when importing, only import RTs with chat_ok=true (default: false)")
    p.add_argument(
        "--oauth",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="convert SSO to RT via pure HTTP (default: true)",
    )
    p.add_argument(
        "--import-sub2api",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="import obtained RTs into Sub2API (default: false)",
    )
    p.add_argument(
        "--base-url",
        default=os.environ.get("SUB2API_BASE_URL", "https://ib.do"),
        help="Sub2API base URL (env SUB2API_BASE_URL)",
    )
    p.add_argument(
        "--admin-token",
        default=os.environ.get("SUB2API_ADMIN_TOKEN", ""),
        help="Sub2API admin token (env SUB2API_ADMIN_TOKEN)",
    )
    p.add_argument("--name-prefix", default="Grok SSO→RT")
    p.add_argument("--group-ids", default="", help="comma-separated group ids, e.g. 18")
    p.add_argument("--client-id", default=DEFAULT_CLIENT_ID)
    p.add_argument("--redirect-uri", default=DEFAULT_REDIRECT_URI)
    p.add_argument("--authorize-url", default=DEFAULT_AUTHORIZE_URL)
    p.add_argument("--limit", type=int, default=0)
    p.add_argument("--sleep", type=float, default=0.5)
    p.add_argument(
        "--write-sso-inventory",
        action="store_true",
        help="also write valid SSO list for grok2api-style tools",
    )
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_arg_parser().parse_args(argv)
    accounts = parse_account_lines(load_input_text(args.input))
    if args.limit and args.limit > 0:
        accounts = accounts[: args.limit]
    if not accounts:
        print("no accounts parsed", file=sys.stderr)
        return 2

    group_ids = [int(x) for x in args.group_ids.split(",") if x.strip()]
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    results: list[AccountResult] = []
    refresh_tokens: list[str] = []
    token_email: dict[str, str] = {}

    print(f"parsed {len(accounts)} account(s)")
    print("mode: pure HTTP SSO -> OAuth RT (no browser)")

    for acc in accounts:
        result = AccountResult(
            line_no=acc.line_no,
            email=acc.email,
            sso_preview=preview_token(acc.sso),
        )
        header = decode_jwt_part(acc.sso.split(".")[0]) if "." in acc.sso else None
        if header is not None:
            result.extras["sso_jwt_typ"] = header.get("typ")
            result.extras["sso_jwt_alg"] = header.get("alg")

        if args.probe_sso:
            try:
                ok, detail = probe_sso(acc.sso)
            except Exception as exc:
                ok, detail = None, f"probe exception: {exc}"
            result.sso_valid = ok
            result.sso_probe_detail = detail
            print(f"[{acc.line_no}] {acc.email} sso_probe={ok} {detail}")
        else:
            print(f"[{acc.line_no}] {acc.email} sso_probe=skipped")

        if args.oauth:
            if result.sso_valid is False:
                result.oauth_status = "skipped"
                result.oauth_detail = "SSO probe failed; skip OAuth"
            else:
                status, detail = "error", ""
                rt = at = ""
                for attempt in range(1, 3):
                    try:
                        rt, at, token = sso_to_oauth_tokens_http(
                            acc.sso,
                            client_id=args.client_id,
                            redirect_uri=args.redirect_uri,
                            authorize_url=args.authorize_url,
                        )
                        status = "ok"
                        detail = json.dumps(
                            {
                                "mode": "http",
                                "expires_in": token.get("expires_in"),
                                "scope": token.get("scope"),
                                "has_refresh_token": bool(rt),
                                "has_access_token": bool(at),
                            },
                            ensure_ascii=False,
                        )
                        break
                    except Exception as exc:
                        status, detail = "error", f"http oauth failed: {exc}"
                        if attempt < 2:
                            print(f"[{acc.line_no}] {acc.email} oauth retry after error: {str(exc)[:120]}")
                            time.sleep(1.2)
                result.oauth_status = status
                result.oauth_detail = detail
                result.refresh_token = rt
                result.access_token = at
                if status == "ok" and rt:
                    if args.probe_rt:
                        try:
                            usable, pdetail, pmeta = probe_refresh_token_usability(
                                rt, client_id=args.client_id
                            )
                        except Exception as exc:
                            usable, pdetail, pmeta = False, f"probe exception: {exc}", {}
                        result.rt_usable = usable
                        result.rt_probe_status = "ok" if usable else "failed"
                        result.rt_probe_detail = pdetail
                        result.extras["rt_probe"] = pmeta
                        print(f"[{acc.line_no}] {acc.email} rt_probe={result.rt_probe_status} {pdetail}")
                        chat_ok = bool(pmeta.get("chat_ok"))
                        if args.import_only_chat_ok and not chat_ok:
                            print(f"[{acc.line_no}] {acc.email} skip import queue (chat_ok=false)")
                        else:
                            refresh_tokens.append(rt)
                            token_email[rt] = acc.email
                    else:
                        result.rt_usable = None
                        result.rt_probe_status = "skipped"
                        refresh_tokens.append(rt)
                        token_email[rt] = acc.email
                    print(f"[{acc.line_no}] {acc.email} oauth=ok rt={preview_token(rt)}")
                else:
                    print(f"[{acc.line_no}] {acc.email} oauth={status} {detail[:180]}")
        else:
            result.oauth_status = "skipped"
            result.oauth_detail = "pass --oauth to convert SSO to RT"

        results.append(result)
        if args.sleep > 0:
            time.sleep(args.sleep)

    import_summary: dict[str, Any] | None = None
    if args.import_sub2api:
        if not args.admin_token:
            print("ERROR: --import-sub2api requires --admin-token / SUB2API_ADMIN_TOKEN", file=sys.stderr)
            return 2
        if not refresh_tokens:
            print("no refresh_tokens obtained; skip Sub2API import")
        else:
            print(f"importing {len(refresh_tokens)} refresh_token(s) into {args.base_url} ...")
            try:
                import_summary = import_refresh_tokens(
                    args.base_url,
                    args.admin_token,
                    refresh_tokens,
                    name_prefix=args.name_prefix,
                    group_ids=group_ids,
                    notes="imported via tools/grok_sso_to_sub2api.py (SSO->RT HTTP)",
                )
                oauth_ok = [r for r in results if r.refresh_token]
                line_results = import_summary.get("results") or []
                for idx, r in enumerate(oauth_ok):
                    if idx >= len(line_results):
                        break
                    item = line_results[idx]
                    if not isinstance(item, dict):
                        continue
                    if item.get("created"):
                        r.import_status = "created"
                        r.account_id = item.get("account_id")
                        r.import_detail = f"account_id={r.account_id}"
                    else:
                        r.import_status = "failed"
                        r.import_detail = str(item.get("error") or item)
                print(
                    "import done:",
                    f"total={import_summary.get('total')} "
                    f"created={import_summary.get('created')} "
                    f"failed={import_summary.get('failed')}",
                )
            except Exception as exc:
                print(f"import failed: {exc}", file=sys.stderr)
                for r in results:
                    if r.refresh_token and r.import_status == "skipped":
                        r.import_status = "failed"
                        r.import_detail = str(exc)

    report = {
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "counts": {
            "accounts": len(results),
            "sso_valid": sum(1 for r in results if r.sso_valid is True),
            "sso_invalid": sum(1 for r in results if r.sso_valid is False),
            "oauth_ok": sum(1 for r in results if r.oauth_status == "ok"),
            "rt_usable": sum(1 for r in results if r.rt_usable is True),
            "rt_chat_ok": sum(1 for r in results if (r.extras.get("rt_probe") or {}).get("chat_ok")),
            "refresh_tokens": len(refresh_tokens),
            "imported_created": sum(1 for r in results if r.import_status == "created"),
        },
        "notes": [
            "SSO is converted to OAuth RT via pure HTTP consent POST (decision=allow).",
            "Sub2API import only accepts RT / typ=at+jwt; never import raw SSO as Bearer.",
        ],
        "results": [
            {
                **{k: v for k, v in asdict(r).items() if k not in {"refresh_token", "access_token"}},
                "refresh_token_preview": preview_token(r.refresh_token) if r.refresh_token else "",
                "access_token_preview": preview_token(r.access_token) if r.access_token else "",
                "has_refresh_token": bool(r.refresh_token),
            }
            for r in results
        ],
        "import_summary": _redact_import_summary(import_summary),
    }
    write_json(out_dir / "report.json", report)

    secrets_payload = {
        "refresh_tokens": [{"email": token_email.get(rt, ""), "refresh_token": rt} for rt in refresh_tokens],
        "accounts": [
            {
                "email": r.email,
                "refresh_token": r.refresh_token,
                "access_token": r.access_token,
                "sso": next((a.sso for a in accounts if a.line_no == r.line_no), ""),
                "password": next((a.password for a in accounts if a.line_no == r.line_no), ""),
            }
            for r in results
        ],
    }
    write_json(out_dir / "secrets.json", secrets_payload)

    if refresh_tokens:
        rt_path = out_dir / "refresh_tokens.txt"
        rt_path.write_text("\n".join(refresh_tokens) + "\n", encoding="utf-8")
        try:
            os.chmod(rt_path, 0o600)
        except OSError:
            pass

    if args.write_sso_inventory:
        valid = [a.sso for a, r in zip(accounts, results) if r.sso_valid is not False]
        inv = out_dir / "sso_tokens_for_grok2api.txt"
        inv.write_text("\n".join(valid) + ("\n" if valid else ""), encoding="utf-8")
        try:
            os.chmod(inv, 0o600)
        except OSError:
            pass

    print(f"report: {out_dir / 'report.json'}")
    print(f"secrets: {out_dir / 'secrets.json'}")
    if not refresh_tokens:
        print("No refresh_tokens obtained.")
        return 1
    print(f"refresh_tokens: {out_dir / 'refresh_tokens.txt'} ({len(refresh_tokens)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
