import { describe, expect, it } from 'vitest'
import { apiReasoningMultipliersToForm, formReasoningMultipliersToAPI } from './types'

describe('reasoning effort pricing form conversion', () => {
  it('keeps only positive configured multipliers', () => {
    expect(formReasoningMultipliersToAPI({ high: '2', max: 3, low: '', none: 0 })).toEqual({ high: 2, max: 3 })
  })

  it('copies API values without sharing the input object', () => {
    const input = { high: 2 }
    const form = apiReasoningMultipliersToForm(input)
    form.high = 3
    expect(input.high).toBe(2)
  })
})
