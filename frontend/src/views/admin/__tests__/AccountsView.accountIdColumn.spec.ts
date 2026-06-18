import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../..')
const accountsView = readFileSync(resolve(root, 'views/admin/AccountsView.vue'), 'utf8')
const zhLocale = readFileSync(resolve(root, 'i18n/locales/zh.ts'), 'utf8')
const enLocale = readFileSync(resolve(root, 'i18n/locales/en.ts'), 'utf8')

describe('AccountsView account id column', () => {
  it('shows account id for log correlation', () => {
    expect(accountsView).toContain("key: 'id', label: t('admin.accounts.columns.id'), sortable: true")
    expect(accountsView).toContain('#{{ value }}')
    expect(zhLocale).toContain("id: '账号ID'")
    expect(enLocale).toContain("id: 'Account ID'")
  })
})
