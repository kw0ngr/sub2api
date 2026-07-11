#!/usr/bin/env python3
"""Grok 卡密全自动导入 Sub2API

从 txt 读取卖家卡密，纯 HTTP 完成：

  解析卡密 → 探测 SSO → SSO 换 OAuth RT → 探测 RT 可用性 → 导入 Sub2API

卡密行格式（任选一种分隔符 | 或 ----）：
  email|password|sso|optional_time
  email----password----sso----optional_time
  sso=eyJ...
  sso:eyJ...

示例：
  python3 tools/grok_sso_to_sub2api.py -i cards.txt --group-ids 18

环境变量：
  SUB2API_BASE_URL      默认 https://ib.do
  SUB2API_ADMIN_TOKEN   管理后台 JWT（必填，用于导入）
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import http.cookiejar
import json
import logging
import os
import re
import secrets
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, TextIO

DEFAULT_CLIENT_ID = "b1a00492-073a-47ea-816f-4c329264a828"
DEFAULT_SCOPE = "openid profile email offline_access grok-cli:access api:access"
DEFAULT_AUTHORIZE_URL = "https://auth.x.ai/oauth2/authorize"
DEFAULT_TOKEN_URL = "https://auth.x.ai/oauth2/token"
DEFAULT_REDIRECT_URI = "http://127.0.0.1:56121/callback"
DEFAULT_BASE_URL = os.environ.get("SUB2API_BASE_URL", "https://ib.do")
DEFAULT_UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/136.0.0.0 Safari/537.36"
)

log = logging.getLogger("grok_sso_import")


# ---------------------------------------------------------------------------
# Models
# ---------------------------------------------------------------------------


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
    elapsed_ms: int = 0
    extras: dict[str, Any] = field(default_factory=dict)


# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------


class _ColorFormatter(logging.Formatter):
    COLORS = {
        logging.DEBUG: "\033[90m",
        logging.INFO: "\033[36m",
        logging.WARNING: "\033[33m",
        logging.ERROR: "\033[31m",
    }
    RESET = "\033[0m"

    def __init__(self, use_color: bool = True) -> None:
        super().__init__("%(asctime)s %(levelname)-5s %(message)s", "%H:%M:%S")
        self.use_color = use_color and sys.stderr.isatty()

    def format(self, record: logging.LogRecord) -> str:
        msg = super().format(record)
        if not self.use_color:
            return msg
        color = self.COLORS.get(record.levelno, "")
        return f"{color}{msg}{self.RESET}" if color else msg


def setup_logging(out_dir: Path, verbose: bool = True) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    log_path = out_dir / "run.log"
    root = logging.getLogger()
    root.handlers.clear()
    root.setLevel(logging.DEBUG if verbose else logging.INFO)

    sh = logging.StreamHandler(sys.stderr)
    sh.setLevel(logging.DEBUG if verbose else logging.INFO)
    sh.setFormatter(_ColorFormatter(use_color=True))
    root.addHandler(sh)

    fh = logging.FileHandler(log_path, encoding="utf-8")
    fh.setLevel(logging.DEBUG)
    fh.setFormatter(logging.Formatter("%(asctime)s %(levelname)-5s %(message)s", "%Y-%m-%d %H:%M:%S"))
    root.addHandler(fh)

    # quiet noisy libs
    logging.getLogger("urllib3").setLevel(logging.WARNING)
    return log_path


def step(msg: str) -> None:
    log.info("▸ %s", msg)


def ok(msg: str) -> None:
    log.info("  ✓ %s", msg)


def warn(msg: str) -> None:
    log.warning("  ! %s", msg)


def fail(msg: str) -> None:
    log.error("  ✗ %s", msg)


def detail(msg: str) -> None:
    log.debug("    · %s", msg)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


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
    tok = strings_trim(token)
    m = re.search(r"(?i)(?:^|[;\s])sso=([^;]+)", tok)
    if m:
        return m.group(1).strip()
    lower = tok.lower()
    for prefix in ("sso=", "sso:", "cookie:sso=", "cookie: sso="):
        if lower.startswith(prefix):
            return tok[len(prefix) :].strip()
    return tok


def strings_trim(s: str) -> str:
    return (s or "").strip().strip("\ufeff")


def parse_account_lines(text: str) -> list[AccountLine]:
    accounts: list[AccountLine] = []
    seen: set[str] = set()
    for idx, raw in enumerate(text.splitlines(), start=1):
        line = strings_trim(raw)
        if not line or line.startswith("#") or line.startswith("//"):
            continue
        # strip seller instruction noise if pasted whole
        if "卡密" in line and "|" not in line and "----" not in line and "eyJ" not in line:
            continue

        email, password, sso, timestamp = "", "", "", ""

        if "----" in line and line.count("----") >= 2:
            parts = [p.strip() for p in line.split("----")]
        elif "|" in line:
            parts = [p.strip() for p in line.split("|")]
        else:
            # bare sso / sso=
            sso = strip_sso_prefix(line)
            if not sso or sso.count(".") < 2:
                log.warning("line %d: skip unparsable: %s", idx, line[:80])
                continue
            parts = ["", "", sso]

        if len(parts) >= 3:
            email, password, sso = parts[0], parts[1], strip_sso_prefix(parts[2])
            timestamp = parts[3] if len(parts) >= 4 else ""
        elif len(parts) == 1:
            sso = strip_sso_prefix(parts[0])
        else:
            log.warning("line %d: skip (need email|password|sso): %s", idx, line[:80])
            continue

        sso = strip_sso_prefix(sso)
        if not sso:
            log.warning("line %d: empty sso, skip", idx)
            continue
        if sso in seen:
            log.warning("line %d: duplicate sso, skip (%s)", idx, email or preview_token(sso))
            continue
        seen.add(sso)
        if not email:
            email = f"sso-line-{idx}"
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


# ---------------------------------------------------------------------------
# SSO probe / OAuth convert / RT probe / import
# ---------------------------------------------------------------------------


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
            time.sleep(0.5 * attempt)
            continue
        text = body.decode("utf-8", errors="replace")
        if status == 200:
            try:
                obj = json.loads(text)
                return True, f"ok remaining={obj.get('remainingQueries')}/{obj.get('totalQueries')} window={obj.get('windowSizeSeconds')}s"
            except Exception:
                return True, "ok"
        if status in (401, 403):
            return False, f"auth failed HTTP {status}: {text[:160]}"
        if status in (429, 502, 503, 520, 521, 522, 523, 524):
            last_err = f"transient HTTP {status}: {text[:160]}"
            time.sleep(0.6 * attempt)
            continue
        return False, f"unexpected HTTP {status}: {text[:160]}"
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
        raise RuntimeError(f"token exchange HTTP {status}: {text[:300]}")
    obj = json.loads(text)
    if not isinstance(obj, dict):
        raise RuntimeError("token exchange returned non-object JSON")
    return obj


def sso_to_oauth_tokens_http(
    sso: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    redirect_uri: str = DEFAULT_REDIRECT_URI,
    authorize_url: str = DEFAULT_AUTHORIZE_URL,
    timeout: float = 30.0,
) -> tuple[str, str, dict[str, Any]]:
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

    detail("warm accounts.x.ai session with SSO cookie")
    _open("GET", "https://accounts.x.ai/sign-in?redirect=grok-com&email=true")

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
    detail("GET authorize (PKCE)")
    status, _, body, final_url = _open("GET", auth_get_url, headers={"Referer": "https://grok.com/"})
    html = body.decode("utf-8", errors="replace")
    detail(f"authorize status={status} final={str(final_url)[:100]}")

    if captured["url"] and "code=" in captured["url"]:
        cb = captured["url"]
        detail("got code without consent page")
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
                f"consent form not found (status={status}, url={str(final_url)[:120]}, head={html[:120]!r})"
            )
        action_m = re.search(r'<form[^>]+action="([^"]+)"', html, flags=re.I)
        action = action_m.group(1) if action_m else "https://auth.x.ai/oauth2/authorize"
        payload = dict(fields)
        payload.setdefault("response_type", "code")
        payload.setdefault("plan", "generic")
        payload["decision"] = "allow"
        referer = final_url if isinstance(final_url, str) and final_url.startswith("http") else auth_get_url
        captured["url"] = None
        detail("POST consent decision=allow")
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
                f"consent POST no code (status={status2}, final={final2!r}, body={body2[:160]!r})"
            )
        detail(f"consent status={status2} got callback")

    values = urllib.parse.parse_qs(urllib.parse.urlparse(cb).query)
    code = (values.get("code") or [""])[0]
    got_state = (values.get("state") or [""])[0]
    if not code:
        raise RuntimeError(f"callback missing code: {cb[:200]}")
    if got_state and got_state != state:
        raise RuntimeError("oauth state mismatch")

    detail("exchange authorization_code for tokens")
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
    detail(f"token expires_in={token.get('expires_in')} scope={token.get('scope')}")
    return rt, at, token


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
        raise RuntimeError(f"refresh HTTP {status}: {text[:300]}")
    obj = json.loads(text)
    if not isinstance(obj, dict) or not str(obj.get("access_token") or "").strip():
        raise RuntimeError(f"refresh missing access_token: {text[:200]}")
    return obj


def probe_refresh_token_usability(
    refresh_token: str,
    *,
    client_id: str = DEFAULT_CLIENT_ID,
    timeout: float = 30.0,
) -> tuple[bool, str, dict[str, Any]]:
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
    try:
        hdr = decode_jwt_part(at.split(".")[0]) or {}
        meta["access_token_typ"] = str(hdr.get("typ") or "")
        meta["access_token_alg"] = str(hdr.get("alg") or "")
    except Exception:
        pass

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
        if 200 <= st < 300 or st == 429:
            meta["chat_ok"] = True
        else:
            msg = body.decode("utf-8", errors="replace")
            meta["chat_error"] = msg[:220]
            low = msg.lower()
            if st == 400 and ("model" in low or "invalid" in low) and "permission" not in low and "incorrect api key" not in low:
                meta["chat_ok"] = True
                meta["chat_note"] = "auth accepted; model/param issue"
    except Exception as exc:
        meta["chat_error"] = str(exc)

    usable = bool(meta["refresh_ok"])
    detail_s = (
        f"refresh_ok={meta['refresh_ok']} typ={meta.get('access_token_typ') or '?'} "
        f"models_ok={meta['models_ok']}({meta.get('models_status')}) "
        f"chat_ok={meta['chat_ok']}({meta.get('chat_status')})"
    )
    if meta.get("chat_error"):
        detail_s += f" chat_err={meta['chat_error'][:100]}"
    return usable, detail_s, meta


def import_refresh_tokens(
    base_url: str,
    admin_token: str,
    refresh_tokens: list[str],
    *,
    name_prefix: str,
    group_ids: list[int],
    notes: str | None = None,
    concurrency: int = 3,
    import_concurrency: int = 5,
    confirm_mixed_channel_risk: bool = True,
    timeout: float = 120.0,
) -> dict[str, Any]:
    url = base_url.rstrip("/") + "/api/v1/admin/grok/oauth/import-refresh-tokens"
    payload: dict[str, Any] = {
        "refresh_tokens": refresh_tokens,
        "import_mode": "refresh_token",
        "name_prefix": name_prefix,
        "concurrency": concurrency if concurrency > 0 else 3,
        "import_concurrency": import_concurrency,
        "confirm_mixed_channel_risk": confirm_mixed_channel_risk,
    }
    if group_ids:
        payload["group_ids"] = group_ids
    if notes:
        payload["notes"] = notes
    detail(f"POST {url} tokens={len(refresh_tokens)} groups={group_ids}")
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
                "concurrency": acc.get("concurrency"),
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


# ---------------------------------------------------------------------------
# Pipeline
# ---------------------------------------------------------------------------


def process_account(
    acc: AccountLine,
    *,
    idx: int,
    total: int,
    args: argparse.Namespace,
) -> AccountResult:
    t0 = time.time()
    result = AccountResult(
        line_no=acc.line_no,
        email=acc.email,
        sso_preview=preview_token(acc.sso),
    )
    header = decode_jwt_part(acc.sso.split(".")[0]) if "." in acc.sso else None
    if header is not None:
        result.extras["sso_jwt_typ"] = header.get("typ")
        result.extras["sso_jwt_alg"] = header.get("alg")

    log.info("======== [%d/%d] line=%d email=%s ========", idx, total, acc.line_no, acc.email)
    detail(f"sso={result.sso_preview} typ={result.extras.get('sso_jwt_typ')} alg={result.extras.get('sso_jwt_alg')}")

    # 1) SSO probe
    if args.probe_sso:
        step("1/4 探测 SSO (grok.com/rest/rate-limits)")
        try:
            ok_flag, pdetail = probe_sso(acc.sso)
        except Exception as exc:
            ok_flag, pdetail = None, f"probe exception: {exc}"
        result.sso_valid = ok_flag
        result.sso_probe_detail = pdetail
        if ok_flag is True:
            ok(f"SSO 有效: {pdetail}")
        elif ok_flag is False:
            fail(f"SSO 无效: {pdetail}")
        else:
            warn(f"SSO 探测不确定: {pdetail}")
    else:
        step("1/4 跳过 SSO 探测")
        result.sso_valid = None
        result.sso_probe_detail = "skipped"

    # 2) SSO -> RT
    if not args.oauth:
        step("2/4 跳过 OAuth 转换")
        result.oauth_status = "skipped"
        result.elapsed_ms = int((time.time() - t0) * 1000)
        return result

    if result.sso_valid is False and not args.force_oauth:
        step("2/4 跳过 OAuth（SSO 无效；可用 --force-oauth 强制尝试）")
        result.oauth_status = "skipped"
        result.oauth_detail = "SSO probe failed"
        result.elapsed_ms = int((time.time() - t0) * 1000)
        return result

    step("2/4 SSO → OAuth RT（纯 HTTP）")
    rt = at = ""
    status, odetail = "error", ""
    for attempt in range(1, args.retries + 1):
        try:
            detail(f"convert attempt {attempt}/{args.retries}")
            rt, at, token = sso_to_oauth_tokens_http(
                acc.sso,
                client_id=args.client_id,
                redirect_uri=args.redirect_uri,
                authorize_url=args.authorize_url,
            )
            status = "ok"
            odetail = json.dumps(
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
            status, odetail = "error", f"http oauth failed: {exc}"
            fail(f"转换失败 attempt={attempt}: {exc}")
            if attempt < args.retries:
                time.sleep(1.0 * attempt)

    result.oauth_status = status
    result.oauth_detail = odetail
    result.refresh_token = rt
    result.access_token = at
    if status == "ok" and rt:
        ok(f"拿到 RT={preview_token(rt)} AT={preview_token(at) if at else '-'}")
    else:
        result.error = odetail
        result.elapsed_ms = int((time.time() - t0) * 1000)
        return result

    # 3) RT probe
    if args.probe_rt:
        step("3/4 探测 RT 可用性 (refresh + models + chat)")
        try:
            usable, pdetail, pmeta = probe_refresh_token_usability(rt, client_id=args.client_id)
        except Exception as exc:
            usable, pdetail, pmeta = False, f"probe exception: {exc}", {}
        result.rt_usable = usable
        result.rt_probe_status = "ok" if usable else "failed"
        result.rt_probe_detail = pdetail
        result.extras["rt_probe"] = pmeta
        if pmeta.get("chat_ok"):
            ok(f"RT 可用: {pdetail}")
        elif usable:
            warn(f"RT 可 refresh 但 chat 可能受限: {pdetail}")
        else:
            fail(f"RT 不可用: {pdetail}")
    else:
        step("3/4 跳过 RT 探测")
        result.rt_probe_status = "skipped"
        result.rt_usable = None

    result.elapsed_ms = int((time.time() - t0) * 1000)
    detail(f"account pipeline done in {result.elapsed_ms}ms")
    return result


def run_pipeline(args: argparse.Namespace) -> int:
    text = Path(args.input).read_text(encoding="utf-8")
    accounts = parse_account_lines(text)
    if args.limit and args.limit > 0:
        accounts = accounts[: args.limit]
    if not accounts:
        log.error("未解析到任何卡密行，请检查 txt 格式")
        return 2

    out_dir = Path(args.out_dir)
    log_path = setup_logging(out_dir, verbose=not args.quiet)
    group_ids = [int(x) for x in args.group_ids.split(",") if x.strip()]

    log.info("============================================================")
    log.info("Grok 卡密全自动导入 Sub2API")
    log.info("============================================================")
    log.info("input     : %s", args.input)
    log.info("accounts  : %d", len(accounts))
    log.info("base_url  : %s", args.base_url)
    log.info("group_ids : %s", group_ids or "(none)")
    log.info("import    : %s", args.import_sub2api)
    log.info("probe_sso : %s  probe_rt: %s  force_oauth: %s", args.probe_sso, args.probe_rt, args.force_oauth)
    log.info("import_only_chat_ok: %s", args.import_only_chat_ok)
    log.info("out_dir   : %s", out_dir)
    log.info("log_file  : %s", log_path)
    if args.import_sub2api and not args.admin_token:
        log.error("开启导入需要 --admin-token 或环境变量 SUB2API_ADMIN_TOKEN")
        return 2

    results: list[AccountResult] = []
    import_queue: list[str] = []
    token_email: dict[str, str] = {}

    for i, acc in enumerate(accounts, start=1):
        result = process_account(acc, idx=i, total=len(accounts), args=args)
        results.append(result)

        # queue for import
        if result.oauth_status == "ok" and result.refresh_token:
            chat_ok = bool((result.extras.get("rt_probe") or {}).get("chat_ok"))
            if args.import_only_chat_ok and result.rt_probe_status != "skipped" and not chat_ok:
                warn(f"{acc.email}: chat_ok=false，跳过导入队列 (--import-only-chat-ok)")
                result.import_status = "skipped"
                result.import_detail = "chat_ok=false"
            else:
                import_queue.append(result.refresh_token)
                token_email[result.refresh_token] = acc.email
                detail(f"queued for import: {acc.email} rt={preview_token(result.refresh_token)}")

        if args.sleep > 0 and i < len(accounts):
            time.sleep(args.sleep)

    # 4) batch import
    import_summary: dict[str, Any] | None = None
    if args.import_sub2api:
        step(f"4/4 导入 Sub2API：{len(import_queue)} 个 RT → {args.base_url}")
        if not import_queue:
            warn("没有可导入的 RT")
        else:
            try:
                import_summary = import_refresh_tokens(
                    args.base_url,
                    args.admin_token,
                    import_queue,
                    name_prefix=args.name_prefix,
                    group_ids=group_ids,
                    concurrency=args.concurrency,
                    notes="imported via tools/grok_sso_to_sub2api.py auto pipeline",
                )
                ok(
                    f"import done total={import_summary.get('total')} "
                    f"created={import_summary.get('created')} failed={import_summary.get('failed')}"
                )
                # map results by order of import_queue
                line_results = import_summary.get("results") or []
                queued_results = [r for r in results if r.refresh_token in token_email and r.import_status != "skipped"]
                # rebuild ordered by import_queue
                by_rt = {r.refresh_token: r for r in results if r.refresh_token}
                for i, rt in enumerate(import_queue):
                    r = by_rt.get(rt)
                    if r is None:
                        continue
                    if i >= len(line_results) or not isinstance(line_results[i], dict):
                        r.import_status = "unknown"
                        continue
                    item = line_results[i]
                    if item.get("created"):
                        r.import_status = "created"
                        r.account_id = item.get("account_id")
                        r.import_detail = f"account_id={r.account_id}"
                        ok(f"created account_id={r.account_id} email={r.email}")
                    else:
                        r.import_status = "failed"
                        r.import_detail = str(item.get("error") or item)
                        fail(f"import failed email={r.email}: {r.import_detail[:160]}")
            except Exception as exc:
                fail(f"import batch failed: {exc}")
                for r in results:
                    if r.refresh_token in token_email and r.import_status not in ("created", "skipped"):
                        r.import_status = "failed"
                        r.import_detail = str(exc)
    else:
        step("4/4 跳过 Sub2API 导入（未开 --import-sub2api / --auto）")

    # summary
    counts = {
        "accounts": len(results),
        "sso_valid": sum(1 for r in results if r.sso_valid is True),
        "sso_invalid": sum(1 for r in results if r.sso_valid is False),
        "oauth_ok": sum(1 for r in results if r.oauth_status == "ok"),
        "rt_usable": sum(1 for r in results if r.rt_usable is True),
        "rt_chat_ok": sum(1 for r in results if (r.extras.get("rt_probe") or {}).get("chat_ok")),
        "imported_created": sum(1 for r in results if r.import_status == "created"),
        "imported_failed": sum(1 for r in results if r.import_status == "failed"),
        "import_queue": len(import_queue),
    }

    log.info("------------------------------------------------------------")
    log.info("汇总")
    log.info("  卡密总数        : %d", counts["accounts"])
    log.info("  SSO 有效        : %d", counts["sso_valid"])
    log.info("  SSO 无效        : %d", counts["sso_invalid"])
    log.info("  换 RT 成功      : %d", counts["oauth_ok"])
    log.info("  RT 可用(refresh): %d", counts["rt_usable"])
    log.info("  RT chat 可用    : %d", counts["rt_chat_ok"])
    log.info("  导入成功        : %d", counts["imported_created"])
    log.info("  导入失败        : %d", counts["imported_failed"])
    log.info("------------------------------------------------------------")
    for r in results:
        log.info(
            "  L%-3d %-36s sso=%-5s oauth=%-7s chat=%-5s import=%-8s id=%s  %dms",
            r.line_no,
            r.email[:36],
            str(r.sso_valid),
            r.oauth_status,
            str((r.extras.get("rt_probe") or {}).get("chat_ok")),
            r.import_status,
            r.account_id or "-",
            r.elapsed_ms,
        )

    report = {
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "input": str(args.input),
        "base_url": args.base_url,
        "group_ids": group_ids,
        "counts": counts,
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
        "refresh_tokens": [
            {"email": token_email.get(rt, ""), "refresh_token": rt} for rt in import_queue
        ],
        "accounts": [
            {
                "email": r.email,
                "line_no": r.line_no,
                "refresh_token": r.refresh_token,
                "access_token": r.access_token,
                "sso": next((a.sso for a in accounts if a.line_no == r.line_no), ""),
                "password": next((a.password for a in accounts if a.line_no == r.line_no), ""),
                "account_id": r.account_id,
            }
            for r in results
        ],
    }
    write_json(out_dir / "secrets.json", secrets_payload)

    if import_queue:
        rt_path = out_dir / "refresh_tokens.txt"
        rt_path.write_text("\n".join(import_queue) + "\n", encoding="utf-8")
        try:
            os.chmod(rt_path, 0o600)
        except OSError:
            pass

    # machine-readable summary line
    summary_path = out_dir / "summary.txt"
    summary_path.write_text(
        "\n".join(
            [
                f"accounts={counts['accounts']}",
                f"sso_valid={counts['sso_valid']}",
                f"oauth_ok={counts['oauth_ok']}",
                f"rt_chat_ok={counts['rt_chat_ok']}",
                f"imported_created={counts['imported_created']}",
                f"imported_failed={counts['imported_failed']}",
            ]
        )
        + "\n",
        encoding="utf-8",
    )

    log.info("输出文件:")
    log.info("  report  : %s", out_dir / "report.json")
    log.info("  secrets : %s", out_dir / "secrets.json")
    log.info("  log     : %s", log_path)
    if import_queue:
        log.info("  rts     : %s (%d)", out_dir / "refresh_tokens.txt", len(import_queue))

    if counts["oauth_ok"] == 0:
        return 1
    if args.import_sub2api and counts["imported_created"] == 0 and import_queue:
        return 1
    return 0


def build_arg_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="从 txt 读取 Grok 卡密，全自动 SSO→RT→探测→导入 Sub2API",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    p.add_argument("-i", "--input", required=True, help="卡密 txt 文件路径（必填）")
    p.add_argument(
        "--out-dir",
        default="",
        help="输出目录（默认 tmp/grok_sso_import_<timestamp>）",
    )
    p.add_argument(
        "--auto",
        action="store_true",
        help="全自动：探测+换RT+探测RT+导入（需 SUB2API_ADMIN_TOKEN）",
    )
    p.add_argument("--probe-sso", action=argparse.BooleanOptionalAction, default=True)
    p.add_argument("--probe-rt", action=argparse.BooleanOptionalAction, default=True)
    p.add_argument("--oauth", action=argparse.BooleanOptionalAction, default=True)
    p.add_argument(
        "--import-sub2api",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="导入 Sub2API（默认：有 admin-token 则开；--auto 强制开）",
    )
    p.add_argument(
        "--import-only-chat-ok",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="仅导入 chat 探测通过的 RT（默认 true）",
    )
    p.add_argument(
        "--force-oauth",
        action="store_true",
        help="即使 SSO 探测失败也尝试换 RT",
    )
    p.add_argument("--base-url", default=DEFAULT_BASE_URL)
    p.add_argument(
        "--admin-token",
        default=os.environ.get("SUB2API_ADMIN_TOKEN", ""),
        help="Sub2API admin JWT（或 env SUB2API_ADMIN_TOKEN）",
    )
    p.add_argument("--name-prefix", default="Grok SSO→RT")
    p.add_argument("--group-ids", default=os.environ.get("SUB2API_GROUP_IDS", "18"))
    p.add_argument("--concurrency", type=int, default=3, help="创建账号 concurrency（默认 3）")
    p.add_argument("--client-id", default=DEFAULT_CLIENT_ID)
    p.add_argument("--redirect-uri", default=DEFAULT_REDIRECT_URI)
    p.add_argument("--authorize-url", default=DEFAULT_AUTHORIZE_URL)
    p.add_argument("--limit", type=int, default=0)
    p.add_argument("--sleep", type=float, default=0.6, help="账号间隔秒数")
    p.add_argument("--retries", type=int, default=2, help="SSO→RT 重试次数")
    p.add_argument("-q", "--quiet", action="store_true", help="少打日志")
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_arg_parser().parse_args(argv)
    if not args.out_dir:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        args.out_dir = f"tmp/grok_sso_import_{ts}"

    # auto mode defaults
    if args.auto:
        args.oauth = True
        args.probe_sso = True
        args.probe_rt = True
        args.import_sub2api = True

    if args.import_sub2api is None:
        # default on when token present
        args.import_sub2api = bool(args.admin_token)

    if not Path(args.input).is_file():
        print(f"input file not found: {args.input}", file=sys.stderr)
        return 2

    return run_pipeline(args)


if __name__ == "__main__":
    raise SystemExit(main())
