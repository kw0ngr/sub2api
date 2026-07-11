#!/usr/bin/env python3
"""Convert bulk Grok account lines into Sub2API OAuth imports.

Input line format (pipe-separated):
  email|password|sso_token|optional_timestamp

Example:
  user@outlook.com|pass123|eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9....|2026-07-11 15:21:40 (GMT+7)

Important protocol note
-----------------------
grok2api uses the SSO cookie against grok.com / console.x.ai (web reverse):

  Cookie: sso=<token>; sso-rw=<token>

Sub2API only accepts official xAI OAuth credentials against api.x.ai:

  Authorization: Bearer <refresh_token or typ=at+jwt access_token>

There is no supported public grant that turns a grok.com SSO cookie into an
OAuth refresh_token. This script therefore:

1. Parses the account dump
2. Probes each SSO against grok.com (same idea as grok2api rate-limits)
3. Optionally tries a headed browser OAuth flow (SSO cookie + password login)
   to capture a PKCE authorization code and exchange it for RT/AT
4. Imports only obtained refresh_tokens into Sub2API admin API

If OAuth cannot be completed automatically, the script still emits a report
and can write SSO inventory for grok2api-style tools (not Sub2API).
"""

from __future__ import annotations

import argparse
import base64
import hashlib
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
from typing import Any, Iterable

# Official Sub2API / xAI CLI OAuth client (see backend/internal/pkg/xai/oauth.go)
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
    # full cookie header paste: "sso=xxx; sso-rw=xxx"
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
        parts = [p.strip() for p in line.split("|")]
        if len(parts) < 3:
            raise ValueError(f"line {idx}: expected email|password|sso|..., got {line!r}")
        email, password, sso = parts[0], parts[1], strip_sso_prefix(parts[2])
        timestamp = parts[3] if len(parts) >= 4 else ""
        if not email or not sso:
            raise ValueError(f"line {idx}: email and sso are required")
        key = sso
        if key in seen:
            continue
        seen.add(key)
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
            body = resp.read()
            return resp.status, {k: v for k, v in resp.headers.items()}, body
    except urllib.error.HTTPError as err:
        body = err.read() if err.fp is not None else b""
        return err.code, {k: v for k, v in err.headers.items()}, body


def probe_sso(sso: str, timeout: float = 20.0) -> tuple[bool, str]:
    """Probe SSO the way grok2api does: Cookie on grok.com rate-limits."""
    status, _, body = http_json(
        "POST",
        "https://grok.com/rest/rate-limits",
        headers={
            "Content-Type": "application/json",
            "Origin": "https://grok.com",
            "Referer": "https://grok.com/",
            "Cookie": f"sso={sso}; sso-rw={sso}",
            "User-Agent": DEFAULT_UA,
        },
        json_body={"modelName": "fast"},
        timeout=timeout,
    )
    text = body.decode("utf-8", errors="replace")
    if status == 200:
        try:
            obj = json.loads(text)
            remaining = obj.get("remainingQueries")
            total = obj.get("totalQueries")
            return True, f"ok remaining={remaining}/{total}"
        except Exception:
            return True, "ok"
    if status in (401, 403):
        return False, f"auth failed HTTP {status}: {text[:180]}"
    # Cloudflare / transient
    if status in (429, 502, 503, 520, 521, 522, 523, 524):
        return None, f"transient HTTP {status}: {text[:180]}"  # type: ignore[return-value]
    return False, f"unexpected HTTP {status}: {text[:180]}"


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


def try_oauth_with_browser(
    account: AccountLine,
    *,
    client_id: str,
    redirect_uri: str,
    authorize_url: str,
    headless: bool,
    timeout_ms: int = 120_000,
) -> tuple[str, str, str]:
    """Best-effort browser OAuth. Returns (status, detail, refresh_or_empty).

    status:
      - ok: got refresh_token (returned in third value as 'rt\\tcode_verifier' no —
        we return refresh_token directly in third field when ok, and pack access too via detail json)
    Simplified: returns (status, detail, refresh_token)
    """
    try:
        from playwright.sync_api import sync_playwright
    except ImportError as exc:
        return "unavailable", f"playwright not installed: {exc}", ""

    code_verifier = b64url(secrets.token_bytes(32))
    code_challenge = b64url(hashlib.sha256(code_verifier.encode("ascii")).digest())
    state = secrets.token_urlsafe(16)
    nonce = secrets.token_hex(8)
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
    auth_url = f"{authorize_url}?{urllib.parse.urlencode(params)}"

    captured: list[str] = []

    def on_request(request: Any) -> None:
        url = request.url
        if "code=" in url and ("callback" in url or redirect_uri.split("://", 1)[-1] in url):
            captured.append(url)

    try:
        with sync_playwright() as p:
            try:
                browser = p.chromium.launch(channel="chrome", headless=headless)
            except Exception:
                browser = p.chromium.launch(headless=headless)
            context = browser.new_context(user_agent=DEFAULT_UA)
            # Seed SSO cookies on xAI hosts (same cookie shape as grok2api).
            for domain in (".x.ai", "accounts.x.ai", "auth.x.ai", "console.x.ai", "grok.com"):
                try:
                    context.add_cookies(
                        [
                            {
                                "name": "sso",
                                "value": account.sso,
                                "domain": domain,
                                "path": "/",
                                "secure": True,
                                "httpOnly": False,
                            },
                            {
                                "name": "sso-rw",
                                "value": account.sso,
                                "domain": domain,
                                "path": "/",
                                "secure": True,
                                "httpOnly": False,
                            },
                        ]
                    )
                except Exception:
                    continue

            page = context.new_page()
            page.on("request", on_request)
            page.goto(auth_url, wait_until="domcontentloaded", timeout=timeout_ms)
            page.wait_for_timeout(2500)

            # Cloudflare / blocked page
            body_text = ""
            try:
                body_text = page.inner_text("body")
            except Exception:
                pass
            if "you have been blocked" in body_text.lower() or "cf-error" in body_text.lower():
                browser.close()
                return "blocked", "Cloudflare blocked headless/browser access to auth.x.ai", ""

            # If redirected to sign-in, attempt password login.
            if "sign-in" in page.url or page.locator("input[type=password]").count() > 0:
                for sel in (
                    "input[type=email]",
                    "input[name=email]",
                    "input[name=username]",
                    "input[autocomplete=username]",
                    "input[type=text]",
                ):
                    if page.locator(sel).count():
                        page.fill(sel, account.email)
                        break
                if page.locator("input[type=password]").count():
                    page.fill("input[type=password]", account.password)
                clicked = False
                for sel in (
                    "button[type=submit]",
                    "button:has-text('Sign in')",
                    "button:has-text('Log in')",
                    "button:has-text('Continue')",
                    "button:has-text('Next')",
                ):
                    if page.locator(sel).count():
                        page.click(sel)
                        clicked = True
                        break
                if not clicked:
                    browser.close()
                    return (
                        "login_required",
                        f"sign-in page needs manual interaction; current url={page.url}",
                        "",
                    )
                page.wait_for_timeout(4000)

            # Consent / authorize buttons if present
            for sel in (
                "button:has-text('Allow')",
                "button:has-text('Authorize')",
                "button:has-text('Continue')",
                "button:has-text('Accept')",
                "button[type=submit]",
            ):
                try:
                    if page.locator(sel).count():
                        page.click(sel, timeout=2000)
                        page.wait_for_timeout(1500)
                except Exception:
                    pass

            # Wait for redirect capture
            deadline = time.time() + max(5.0, timeout_ms / 1000.0)
            while time.time() < deadline and not captured:
                # also inspect current URL
                if "code=" in page.url and "callback" in page.url:
                    captured.append(page.url)
                    break
                page.wait_for_timeout(500)

            browser.close()
    except Exception as exc:
        return "error", f"browser oauth failed: {exc}", ""

    if not captured:
        return (
            "no_code",
            "OAuth code not captured. SSO cookie alone usually cannot complete "
            "api.x.ai OAuth (accounts.x.ai session is separate from grok.com SSO).",
            "",
        )

    cb = captured[0]
    qs = urllib.parse.urlparse(cb).query
    values = urllib.parse.parse_qs(qs)
    code = (values.get("code") or [""])[0]
    got_state = (values.get("state") or [""])[0]
    if not code:
        return "no_code", f"callback without code: {cb[:200]}", ""
    if got_state and got_state != state:
        return "error", "oauth state mismatch", ""

    try:
        token = exchange_code_for_tokens(
            code,
            code_verifier,
            client_id=client_id,
            redirect_uri=redirect_uri,
        )
    except Exception as exc:
        return "exchange_failed", str(exc), ""

    rt = str(token.get("refresh_token") or "").strip()
    at = str(token.get("access_token") or "").strip()
    if not rt and not at:
        return "exchange_failed", f"token response missing credentials: {token}", ""
    # stash access token in detail json for caller
    detail = json.dumps(
        {
            "has_refresh_token": bool(rt),
            "has_access_token": bool(at),
            "expires_in": token.get("expires_in"),
            "token_type": token.get("token_type"),
            "access_token_preview": preview_token(at) if at else "",
        },
        ensure_ascii=False,
    )
    # encode access token after detail using a reserved field via result extras by caller
    # We return refresh_token primarily; access token appended with \x1e separator if needed.
    packed = rt if rt else ""
    if at:
        packed = f"{packed}\x1e{at}" if packed else f"\x1e{at}"
    return "ok", detail, packed


def import_refresh_tokens(
    base_url: str,
    admin_token: str,
    refresh_tokens: list[str],
    *,
    name_prefix: str,
    group_ids: list[int],
    import_concurrency: int = 5,
    confirm_mixed_channel_risk: bool = True,
    timeout: float = 120.0,
) -> dict[str, Any]:
    url = base_url.rstrip("/") + "/api/v1/admin/grok/oauth/import-refresh-tokens"
    payload = {
        "refresh_tokens": refresh_tokens,
        "import_mode": "refresh_token",
        "name_prefix": name_prefix,
        "import_concurrency": import_concurrency,
        "confirm_mixed_channel_risk": confirm_mixed_channel_risk,
    }
    if group_ids:
        payload["group_ids"] = group_ids
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
    # envelope may be {data: ...} or direct
    if isinstance(obj, dict) and "data" in obj and isinstance(obj["data"], dict):
        return obj["data"]
    if isinstance(obj, dict):
        return obj
    raise RuntimeError(f"unexpected import response: {text[:200]}")


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def load_input_text(path: str | None) -> str:
    if path and path != "-":
        return Path(path).read_text(encoding="utf-8")
    if not sys.stdin.isatty():
        return sys.stdin.read()
    raise SystemExit("provide --input file or pipe account lines via stdin")


def build_arg_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="Parse email|password|sso lines, probe SSO, optionally OAuth to RT, import to Sub2API",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    p.add_argument("--input", "-i", help="account dump file (default: stdin)")
    p.add_argument("--out-dir", default="tmp/grok_sso_import", help="report output directory")
    p.add_argument(
        "--probe-sso",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="probe SSO against grok.com/rest/rate-limits (default: true)",
    )
    p.add_argument(
        "--oauth",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="attempt browser OAuth to obtain refresh_token (default: false)",
    )
    p.add_argument(
        "--headed",
        action="store_true",
        help="run browser OAuth headed (recommended; headless often CF-blocked)",
    )
    p.add_argument(
        "--import-sub2api",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="import obtained refresh_tokens into Sub2API (default: false)",
    )
    p.add_argument(
        "--base-url",
        default=os.environ.get("SUB2API_BASE_URL", "https://ib.do"),
        help="Sub2API base URL (env SUB2API_BASE_URL)",
    )
    p.add_argument(
        "--admin-token",
        default=os.environ.get("SUB2API_ADMIN_TOKEN", ""),
        help="Sub2API admin JWT/API token (env SUB2API_ADMIN_TOKEN)",
    )
    p.add_argument("--name-prefix", default="Grok SSO→RT")
    p.add_argument(
        "--group-ids",
        default="",
        help="comma-separated group ids for import, e.g. 18",
    )
    p.add_argument("--client-id", default=DEFAULT_CLIENT_ID)
    p.add_argument("--redirect-uri", default=DEFAULT_REDIRECT_URI)
    p.add_argument("--authorize-url", default=DEFAULT_AUTHORIZE_URL)
    p.add_argument("--limit", type=int, default=0, help="only process first N accounts")
    p.add_argument("--sleep", type=float, default=0.4, help="delay between accounts")
    p.add_argument(
        "--write-sso-inventory",
        action="store_true",
        help="also write valid SSO list for grok2api-style tools (not importable to Sub2API)",
    )
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_arg_parser().parse_args(argv)
    text = load_input_text(args.input)
    accounts = parse_account_lines(text)
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
    print(
        "note: SSO cookie ≠ OAuth RT. Sub2API only imports RT/at+jwt. "
        "This script probes SSO (grok2api-style) and optionally tries browser OAuth."
    )

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
            ok, detail = probe_sso(acc.sso)
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
                status, detail, packed = try_oauth_with_browser(
                    acc,
                    client_id=args.client_id,
                    redirect_uri=args.redirect_uri,
                    authorize_url=args.authorize_url,
                    headless=not args.headed,
                )
                result.oauth_status = status
                result.oauth_detail = detail
                rt, at = "", ""
                if packed:
                    if "\x1e" in packed:
                        rt, at = packed.split("\x1e", 1)
                    else:
                        rt = packed
                result.refresh_token = rt
                result.access_token = at
                if status == "ok" and rt:
                    refresh_tokens.append(rt)
                    token_email[rt] = acc.email
                    print(f"[{acc.line_no}] {acc.email} oauth=ok rt={preview_token(rt)}")
                else:
                    print(f"[{acc.line_no}] {acc.email} oauth={status} {detail[:160]}")
        else:
            result.oauth_status = "skipped"
            result.oauth_detail = "pass --oauth to attempt browser OAuth for RT"

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
                )
                # map line results
                by_preview = {
                    preview_token(rt): rt for rt in refresh_tokens
                }
                for item in import_summary.get("results") or []:
                    if not isinstance(item, dict):
                        continue
                    # best-effort match by order if previews unavailable
                # Map by order of successful oauth results
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

    # Write reports (never dump full passwords/sso by default; keep redacted)
    report = {
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "counts": {
            "accounts": len(results),
            "sso_valid": sum(1 for r in results if r.sso_valid is True),
            "sso_invalid": sum(1 for r in results if r.sso_valid is False),
            "oauth_ok": sum(1 for r in results if r.oauth_status == "ok"),
            "refresh_tokens": len(refresh_tokens),
            "imported_created": sum(1 for r in results if r.import_status == "created"),
        },
        "notes": [
            "SSO cookies work for grok2api (Cookie sso=... on grok.com/console.x.ai).",
            "Sub2API requires OAuth RT / typ=at+jwt against api.x.ai.",
            "There is no stable public SSO→RT exchange; --oauth is best-effort browser automation.",
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
        "import_summary": import_summary,
    }
    write_json(out_dir / "report.json", report)

    # Full secrets file for operator reuse (chmod advice)
    secrets_payload = {
        "refresh_tokens": [
            {
                "email": token_email.get(rt, ""),
                "refresh_token": rt,
            }
            for rt in refresh_tokens
        ],
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
    secrets_path = out_dir / "secrets.json"
    write_json(secrets_path, secrets_payload)
    try:
        os.chmod(secrets_path, 0o600)
    except OSError:
        pass

    if refresh_tokens:
        (out_dir / "refresh_tokens.txt").write_text(
            "\n".join(refresh_tokens) + "\n", encoding="utf-8"
        )
        try:
            os.chmod(out_dir / "refresh_tokens.txt", 0o600)
        except OSError:
            pass

    if args.write_sso_inventory:
        valid_sso = []
        for acc, r in zip(accounts, results):
            if r.sso_valid is False:
                continue
            valid_sso.append(acc.sso)
        (out_dir / "sso_tokens_for_grok2api.txt").write_text(
            "\n".join(valid_sso) + ("\n" if valid_sso else ""),
            encoding="utf-8",
        )
        try:
            os.chmod(out_dir / "sso_tokens_for_grok2api.txt", 0o600)
        except OSError:
            pass

    print(f"report: {out_dir / 'report.json'}")
    print(f"secrets: {secrets_path} (contains SSO/password/RT; keep private)")
    if not refresh_tokens:
        print(
            "\nNo OAuth refresh_tokens obtained.\n"
            "Next options:\n"
            "  1) Run with: --oauth --headed   (manual CF/login if needed)\n"
            "  2) Use Sub2API browser OAuth flow to get RT, then bulk import RT only\n"
            "  3) Keep SSO for grok2api (--write-sso-inventory); Sub2API cannot use SSO as Bearer\n"
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
