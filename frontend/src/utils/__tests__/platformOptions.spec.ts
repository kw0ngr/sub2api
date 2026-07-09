import { describe, expect, it } from 'vitest'
import { platformFilterOptionsWithAll, supportedPlatformOptions } from '../platformOptions'

describe('platformOptions', () => {
  it('keeps every first-class platform in one shared option list', () => {
    expect(supportedPlatformOptions.map((option) => option.value)).toEqual([
      'anthropic',
      'openai',
      'gemini',
      'glm',
      'grok',
      'antigravity',
      'openrouter',
      'deepseek'
    ])
  })

  it('prepends an all-platform filter option', () => {
    expect(platformFilterOptionsWithAll('All')[0]).toEqual({ value: '', label: 'All' })
    expect(platformFilterOptionsWithAll('All').slice(1)).toEqual(supportedPlatformOptions)
  })
})
