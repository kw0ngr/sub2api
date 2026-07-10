import { describe, expect, it, vi } from 'vitest'

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

import { buildModelMappingObject, getModelsByPlatform, getPresetMappingsByPlatform } from '../useModelWhitelist'

describe('useModelWhitelist', () => {
  it('openai 模型列表包含 GPT-5.6 官方模型', () => {
    const models = getModelsByPlatform('openai')

    expect(models.slice(0, 3)).toEqual(['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna'])
  })

  it('openai 模型列表包含 GPT-5.4 官方快照', () => {
    const models = getModelsByPlatform('openai')

    expect(models).toContain('gpt-5.4')
    expect(models).toContain('gpt-5.4-mini')
    expect(models).toContain('gpt-5.4-2026-03-05')
  })

  it('openai 模型列表不再暴露已下线的 ChatGPT 登录 Codex 模型', () => {
    const models = getModelsByPlatform('openai')

    expect(models).not.toContain('gpt-5')
    expect(models).not.toContain('gpt-5.1')
    expect(models).not.toContain('gpt-5.1-codex')
    expect(models).not.toContain('gpt-5.1-codex-max')
    expect(models).not.toContain('gpt-5.1-codex-mini')
    expect(models).not.toContain('gpt-5.2-codex')
  })

  it('antigravity 模型列表包含图片模型兼容项', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models).toContain('gemini-3-pro-image')
  })

  it('gemini 模型列表包含原生生图模型', () => {
    const models = getModelsByPlatform('gemini')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.0-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
  })

  it('deepseek 模型列表包含 v4 pro/flash', () => {
    const models = getModelsByPlatform('deepseek')

    expect(models).toContain('deepseek-v4-pro')
    expect(models).toContain('deepseek-v4-flash')
  })

  it('glm 模型列表优先包含 5.x 和 4.7 系列', () => {
    const models = getModelsByPlatform('glm')

    expect(models.slice(0, 5)).toEqual([
      'glm-5.2',
      'glm-5-turbo',
      'glm-5',
      'glm-4.7',
      'glm-4.7-flashx'
    ])
    expect(models).toContain('chatglm_turbo')
  })

  it('grok 模型列表包含 xAI 官方别名', () => {
    const models = getModelsByPlatform('grok')

    expect(models.slice(0, 3)).toEqual(['grok-4.5', 'grok-4.3', 'grok-build-0.1'])
    expect(models).toContain('grok-composer-2.5-fast')
    expect(models).toContain('grok-4.20-0309-reasoning')
  })

  it('anthropic/antigravity 模型列表包含新发布的 Claude 模型', () => {
    expect(getModelsByPlatform('anthropic')).toContain('claude-fable-5')
    expect(getModelsByPlatform('antigravity')).toContain('claude-fable-5')
    expect(getModelsByPlatform('anthropic')).toContain('claude-opus-4-8')
    expect(getModelsByPlatform('anthropic')).toContain('claude-sonnet-5')
    expect(getModelsByPlatform('antigravity')).toContain('claude-opus-4-8')
  })

  it('antigravity 和 bedrock 预设映射包含新发布的 Claude 模型', () => {
    expect(getPresetMappingsByPlatform('anthropic')).toContainEqual(expect.objectContaining({
      from: 'claude-sonnet-5',
      to: 'claude-sonnet-5'
    }))
    expect(getPresetMappingsByPlatform('antigravity')).toContainEqual(expect.objectContaining({
      from: 'claude-fable-5',
      to: 'claude-fable-5'
    }))
    expect(getPresetMappingsByPlatform('antigravity')).toContainEqual(expect.objectContaining({
      from: 'claude-opus-4-8',
      to: 'claude-opus-4-8'
    }))
    expect(getPresetMappingsByPlatform('bedrock')).toContainEqual(expect.objectContaining({
      from: 'claude-fable-5',
      to: 'anthropic.claude-fable-5'
    }))
    expect(getPresetMappingsByPlatform('bedrock')).toContainEqual(expect.objectContaining({
      from: 'claude-opus-4-8',
      to: 'us.anthropic.claude-opus-4-8-v1'
    }))
    expect(getPresetMappingsByPlatform('bedrock')).toContainEqual(expect.objectContaining({
      from: 'claude-sonnet-5',
      to: 'us.anthropic.claude-sonnet-5-v1'
    }))
  })

  it('antigravity 模型列表会把新的 Gemini 图片模型排在前面', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash-lite'))
  })

  it('whitelist 模式会忽略通配符条目', () => {
    const mapping = buildModelMappingObject('whitelist', ['claude-*', 'gemini-3.1-flash-image'], [])
    expect(mapping).toEqual({
      'gemini-3.1-flash-image': 'gemini-3.1-flash-image'
    })
  })

  it('whitelist 模式会保留 GPT-5.4 官方快照的精确映射', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4-2026-03-05'], [])

    expect(mapping).toEqual({
      'gpt-5.4-2026-03-05': 'gpt-5.4-2026-03-05'
    })
  })

  it('whitelist keeps GPT-5.4 mini exact mappings', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4-mini'], [])

    expect(mapping).toEqual({
      'gpt-5.4-mini': 'gpt-5.4-mini'
    })
  })
})
