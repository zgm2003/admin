import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, initializeLocale, isAppMessageKey, localeStorageKey, readLocale, setLocale } from '@src/i18n/index'
import { enUS } from '@src/i18n/messages/en-US'
import { zhCN } from '@src/i18n/messages/zh-CN'

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
    expect(Object.keys(enUS).sort()).toEqual(Object.keys(zhCN).sort())
  })

  it('recognizes only exact application message keys', () => {
    expect(isAppMessageKey('navigation.dashboard')).toBe(true)
    expect(isAppMessageKey('access.loadFailed')).toBe(true)
    expect(isAppMessageKey('navigation.unknown')).toBe(false)
  })

	it('contains the complete bilingual user-management copy', () => {
		const keys = [
			'navigation.systemUsers', 'permission.userUpdate', 'permission.userStatus',
			'permission.userDelete', 'permission.userRoles', 'user.title', 'user.keyword',
			'user.status', 'user.role', 'user.search', 'user.reset', 'user.refresh',
			'user.enableConfirm', 'user.disableConfirm', 'user.deleteConfirm',
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
      'navigation.systemAuthPlatforms', 'permission.authPlatformCreate',
      'permission.authPlatformUpdate', 'permission.authPlatformStatus',
      'permission.authPlatformDelete', 'authPlatform.title', 'authPlatform.search',
      'authPlatform.deployment', 'authPlatform.confirm.disable',
    ]
    for (const key of keys) {
      expect(isAppMessageKey(key), key).toBe(true)
      expect(appI18n.global.t(key), key).toBeTruthy()
    }
    setLocale('en-US')
    expect(appI18n.global.t('authPlatform.title')).toBe('Authentication platforms')
  })

	it('contains the complete bilingual session and operation-log copy', () => {
		const keys = [
			'navigation.systemSessions', 'permission.sessionRevoke', 'session.title',
			'session.loading', 'session.batchRevoke', 'session.revokeFailed',
			'navigation.systemOperationLogs', 'operationLog.title', 'operationLog.userId',
			'operationLog.timeRange', 'operationLog.detailTitle', 'operationLog.loading',
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
