export const OPENAI_CC_SWITCH_CODEX_MODEL = "gpt-5.5";

export type CcSwitchClientType = "claude" | "gemini";

export interface CcSwitchImportConfig {
  app: string;
  endpoint: string;
  model?: string;
}

export interface BuildCcSwitchImportDeeplinkInput {
  config: CcSwitchImportConfig;
  providerName: string;
  homepage: string;
  apiKey: string;
  usageScript: string;
}

export function resolveCcSwitchImportConfig(
  platform: string,
  clientType: CcSwitchClientType,
  baseUrl: string,
): CcSwitchImportConfig {
  switch (platform) {
    case "antigravity":
      return {
        app: clientType === "gemini" ? "gemini" : "claude",
        endpoint: `${baseUrl}/antigravity`,
      };
    case "openai":
      return {
        app: "codex",
        endpoint: baseUrl,
        model: OPENAI_CC_SWITCH_CODEX_MODEL,
      };
    case "gemini":
      return {
        app: "gemini",
        endpoint: baseUrl,
      };
    default:
      return {
        app: "claude",
        endpoint: baseUrl,
      };
  }
}

export function buildCcSwitchImportDeeplink(
  input: BuildCcSwitchImportDeeplinkInput,
): string {
  const params = new URLSearchParams({
    resource: "provider",
    app: input.config.app,
    name: input.providerName,
    homepage: input.homepage,
    endpoint: input.config.endpoint,
    apiKey: input.apiKey,
    configFormat: "json",
    usageEnabled: "true",
    usageScript: btoa(input.usageScript),
    usageAutoInterval: "30",
  });

  if (input.config.model) {
    params.set("model", input.config.model);
  }

  return `ccswitch://v1/import?${params.toString()}`;
}
