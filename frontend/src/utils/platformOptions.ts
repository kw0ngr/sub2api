import type { GroupPlatform } from '@/types'

export type PlatformOption = {
  value: GroupPlatform
  label: string
}

export const supportedPlatformOptions: PlatformOption[] = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'glm', label: 'GLM' },
  { value: 'grok', label: 'Grok' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'deepseek', label: 'DeepSeek' }
]

export function platformFilterOptionsWithAll(allLabel: string) {
  return [{ value: '', label: allLabel }, ...supportedPlatformOptions]
}
