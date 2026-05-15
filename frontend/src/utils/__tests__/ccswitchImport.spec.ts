import { describe, expect, it } from "vitest";

import {
  buildCcSwitchImportDeeplink,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  resolveCcSwitchImportConfig,
} from "../ccswitchImport";

describe("ccswitch import deeplink", () => {
  it("adds codex app and model for OpenAI imports", () => {
    const config = resolveCcSwitchImportConfig("openai", "claude", "https://api.example.test");

    expect(config).toEqual({
      app: "codex",
      endpoint: "https://api.example.test",
      model: OPENAI_CC_SWITCH_CODEX_MODEL,
    });

    const deeplink = buildCcSwitchImportDeeplink({
      config,
      providerName: "sub2api",
      homepage: "https://api.example.test",
      apiKey: "sk-test",
      usageScript: "({ request: {} })",
    });
    const parsed = new URL(deeplink);

    expect(parsed.protocol).toBe("ccswitch:");
    expect(parsed.searchParams.get("app")).toBe("codex");
    expect(parsed.searchParams.get("model")).toBe(OPENAI_CC_SWITCH_CODEX_MODEL);
    expect(parsed.searchParams.get("endpoint")).toBe("https://api.example.test");
    expect(parsed.searchParams.get("apiKey")).toBe("sk-test");
  });

  it("keeps existing platform endpoint behavior", () => {
    expect(resolveCcSwitchImportConfig("anthropic", "claude", "https://api.example.test")).toEqual({
      app: "claude",
      endpoint: "https://api.example.test",
    });
    expect(resolveCcSwitchImportConfig("gemini", "gemini", "https://api.example.test")).toEqual({
      app: "gemini",
      endpoint: "https://api.example.test",
    });
    expect(resolveCcSwitchImportConfig("antigravity", "gemini", "https://api.example.test")).toEqual({
      app: "gemini",
      endpoint: "https://api.example.test/antigravity",
    });
  });
});
