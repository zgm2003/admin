import { beforeEach, describe, expect, it } from 'vitest'

import {
  appI18n,
  initializeLocale,
  isAppMessageKey,
  localeStorageKey,
  readLocale,
  setLocale,
} from '@/i18n/index'
import { enUS } from '@/i18n/messages/en-US'
import { zhCN } from '@/i18n/messages/zh-CN'

describe('frontend i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.lang = ''
    setLocale('zh-CN')
  })

  it('defaults to Chinese and updates the root language', () => {
    expect(initializeLocale()).toBe('zh-CN')
    expect(readLocale()).toBe('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
    expect(appI18n.global.t('navigation.dashboard')).toBe('工作台')
  })
  it('normalizes an invalid stored locale to Chinese', () => {
    localStorage.setItem(localeStorageKey, 'fr-FR')
    expect(initializeLocale()).toBe('zh-CN')
    expect(localStorage.getItem(localeStorageKey)).toBe('zh-CN')
  })

  it('persists English and keeps both catalogs exactly shaped', () => {
    setLocale('en-US')
    expect(localStorage.getItem(localeStorageKey)).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(appI18n.global.t('navigation.dashboard')).toBe('Dashboard')
    expect(appI18n.global.t('navigation.messageNotification')).toBe('Notification center')
    expect(Object.keys(enUS).sort()).toEqual(Object.keys(zhCN).sort())
  })

  it('translates the hidden notification center route', () => {
    expect(appI18n.global.t('navigation.messageNotification')).toBe('通知中心')
  })

  it('recognizes only exact application message keys', () => {
    expect(isAppMessageKey('navigation.dashboard')).toBe(true)
    expect(isAppMessageKey('access.loadFailed')).toBe(true)
    expect(isAppMessageKey('navigation.unknown')).toBe(false)
  })

  it('translates the shared status label in both languages', () => {
    for (const locale of ['zh-CN', 'en-US'] as const) {
      setLocale(locale)
      expect(appI18n.global.t('common.status')).toBe(locale === 'zh-CN' ? '状态' : 'Status')
    }
  })

  it('uses only two shared enable and disable keys for actions and statuses', () => {
    for (const locale of ['zh-CN', 'en-US'] as const) {
      setLocale(locale)
      expect(appI18n.global.t('common.enable')).toBe(locale === 'zh-CN' ? '启用' : 'Enable')
      expect(appI18n.global.t('common.disable')).toBe(locale === 'zh-CN' ? '停用' : 'Disable')
      expect(isAppMessageKey('common.enabled')).toBe(false)
      expect(isAppMessageKey('common.disabled')).toBe(false)
    }
  })

  it('provides shared labels for stable cross-page actions and table metadata', () => {
    const expected = {
      'common.status': ['状态', 'Status'],
      'common.actions': ['操作', 'Actions'],
      'common.edit': ['编辑', 'Edit'],
      'common.save': ['保存', 'Save'],
      'common.cancel': ['取消', 'Cancel'],
      'common.close': ['关闭', 'Close'],
      'common.delete': ['删除', 'Delete'],
      'common.retry': ['重试', 'Retry'],
      'common.refresh': ['刷新', 'Refresh'],
      'common.search': ['查询', 'Search'],
      'common.reset': ['重置', 'Reset'],
      'common.detail': ['详情', 'Details'],
      'common.remark': ['备注', 'Remark'],
      'common.createdAt': ['创建时间', 'Created at'],
      'common.updatedAt': ['更新时间', 'Updated at'],
      'common.sortOrder': ['排序', 'Sort order'],
      'common.allStatuses': ['全部状态', 'All statuses'],
      'common.expand': ['展开', 'Expand'],
      'common.collapse': ['收起', 'Collapse'],
      'common.selectAll': ['全选', 'Select all'],
      'common.clear': ['清空', 'Clear'],
      'common.download': ['下载', 'Download'],
      'common.preview': ['预览', 'Preview'],
      'common.confirm': ['确认', 'Confirm'],
    } as const

    for (const [locale, index] of [
      ['zh-CN', 0],
      ['en-US', 1],
    ] as const) {
      setLocale(locale)
      for (const [key, values] of Object.entries(expected)) {
        expect(isAppMessageKey(key), key).toBe(true)
        expect(appI18n.global.t(key), key).toBe(values[index])
      }
    }
  })

  it('contains only the current business-domain navigation keys', () => {
    const currentKeys = [
      'navigation.user',
      'navigation.userAccount',
      'navigation.userSession',
      'navigation.permission',
      'navigation.permissionMenu',
      'navigation.permissionRole',
      'navigation.permissionAuthPlatform',
      'navigation.system',
      'navigation.systemOperationLog',
      'navigation.sms',
    ]
    const removedKeys = [
      'navigation.systemUsers',
      'navigation.systemSessions',
      'navigation.systemMenus',
      'navigation.systemRoles',
      'navigation.systemAuthPlatforms',
    ]
    for (const key of currentKeys) expect(isAppMessageKey(key), key).toBe(true)
    for (const key of removedKeys) expect(isAppMessageKey(key), key).toBe(false)
  })

  it('contains the complete bilingual user-management copy', () => {
    const keys = [
      'navigation.userAccount',
      'permission.userUpdate',
      'permission.userStatus',
      'permission.userDelete',
      'permission.userRoles',
      'user.title',
      'user.keyword',
      'common.status',
      'user.role',
      'common.search',
      'common.reset',
      'common.refresh',
      'user.enableConfirm',
      'user.disableConfirm',
      'user.deleteConfirm',
    ]
    for (const key of keys) {
      expect(isAppMessageKey(key)).toBe(true)
      expect(appI18n.global.t(key), key).toBeTruthy()
    }
    expect(appI18n.global.t('user.disableConfirm')).toContain('重新登录')
    expect(appI18n.global.t('user.deleteConfirm')).toContain('新账号')
    setLocale('en-US')
    expect(appI18n.global.t('user.disableConfirm')).toContain('sign in')
    expect(appI18n.global.t('user.deleteConfirm')).toContain('new account')
  })

  it('contains the complete bilingual authentication-platform copy', () => {
    const keys = [
      'navigation.permissionAuthPlatform',
      'permission.authPlatformCreate',
      'permission.authPlatformUpdate',
      'permission.authPlatformStatus',
      'permission.authPlatformDelete',
      'permission.authPlatform.title',
      'common.search',
      'permission.authPlatform.form.basicSection',
      'permission.authPlatform.form.tokenSection',
      'permission.authPlatform.form.policySection',
      'permission.authPlatform.confirm.disable',
    ]
    for (const key of keys) {
      expect(isAppMessageKey(key), key).toBe(true)
      expect(appI18n.global.t(key), key).toBeTruthy()
    }
    setLocale('en-US')
    expect(appI18n.global.t('permission.authPlatform.title')).toBe('Authentication platforms')
  })

  it('contains the complete bilingual session and operation-log copy', () => {
    const keys = [
      'navigation.userSession',
      'permission.sessionRevoke',
      'session.title',
      'session.loading',
      'session.batchRevoke',
      'session.revokeFailed',
      'navigation.systemOperationLog',
      'operationLog.title',
      'operationLog.userId',
      'operationLog.timeRange',
      'operationLog.detailTitle',
      'operationLog.loading',
    ]
    for (const key of keys) {
      expect(isAppMessageKey(key), key).toBe(true)
      expect(appI18n.global.t(key), key).toBeTruthy()
    }
    setLocale('en-US')
    expect(appI18n.global.t('session.title')).toBe('Session management')
    expect(appI18n.global.t('operationLog.title')).toBe('Operation logs')
  })
})
