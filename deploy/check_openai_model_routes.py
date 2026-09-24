#!/usr/bin/env python3
"""Release gate: a group key must list and actually stream each new OpenAI model."""

import json
import os
import sys
import urllib.error
import urllib.request

base = os.environ.get("SUB2API_BASE_URL", "").rstrip("/")
key = os.environ.get("SUB2API_API_KEY", "")
models = sys.argv[1:]
if not base or not key or not models:
    sys.exit("Usage: SUB2API_BASE_URL=https://host SUB2API_API_KEY=... check_openai_model_routes.py MODEL [MODEL ...]")

headers = {"Authorization": "Bearer " + key}


def request(path, data=None):
    request_headers = dict(headers)
    if data is not None:
        request_headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base + path, data=data, headers=request_headers)
    return urllib.request.urlopen(req, timeout=120)


try:
    with request("/v1/models") as resp:
        listed = {model["id"] for model in json.load(resp)["data"]}
    with request("/backend-api/codex/models") as resp:
        codex = {model["slug"] for model in json.load(resp)["models"]}
    for model in models:
        if model not in listed or model not in codex:
            raise ValueError(f"{model}: missing from /v1/models or Codex manifest")
        payload = json.dumps({
            "model": model, "input": "Reply exactly OK.", "stream": True,
            "reasoning": {"effort": "low"}, "max_output_tokens": 128,
        }).encode()
        completed = False
        has_text = False
        with request("/v1/responses", payload) as resp:
            for line in resp:
                if line.startswith(b"data: "):
                    event = json.loads(line[6:]) if line[6:].strip() != b"[DONE]" else {}
                    if event.get("type") == "response.output_text.delta" and event.get("delta"):
                        has_text = True
                    if event.get("type") == "response.completed":
                        response = event.get("response", {})
                        completed = response.get("status") == "completed"
                        has_text |= any(part.get("text") for item in response.get("output", [])
                                        for part in item.get("content", []) if part.get("type") == "output_text")
                    if event.get("type") == "response.failed":
                        raise ValueError(f"{model}: upstream response.failed")
        if not completed or not has_text:
            raise ValueError(f"{model}: stream did not complete with output text")
        print(f"{model}: listed + Codex manifest + streaming output")
except (urllib.error.URLError, ValueError, KeyError, json.JSONDecodeError) as exc:
    sys.exit(f"model release gate failed: {exc}")
