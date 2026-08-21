import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as sessionAPI from '../../../api/session'
import type { SessionItem } from '../../../api/session.contract'
import { appI18n, setLocale } from '../../../i18n'
import { useAccessStore } from '../../../store/access'
import SessionManagement from './index.vue'

vi.mock('../../../api/session', () => ({
	getSessions: vi.fn(),
	getSessionStats: vi.fn(),
	revokeSession: vi.fn(),
	revokeSessions: vi.fn(),
}))

const getSessions = vi.mocked(sessionAPI.getSessions)
const getSessionStats = vi.mocked(sessionAPI.getSessionStats)
const revokeSession = vi.mocked(sessionAPI.revokeSession)
const revokeSessions = vi.mocked(sessionAPI.revokeSessions)

describe('session management', () => {
	beforeEach(() => {
		vi.clearAllMocks()
		setLocale('zh-CN')
		getSessions.mockResolvedValue({ list: rows(), total: 2, page: 1, pageSize: 20 })
		getSessionStats.mockResolvedValue({ activeTotal: 2, platforms: { admin: 2 } })
		revokeSession.mockResolvedValue({ revoked: 1, skippedCurrent: 0, skippedRevoked: 0 })
		revokeSessions.mockResolvedValue({ revoked: 1, skippedCurrent: 0, skippedRevoked: 0 })
		vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () =>
			Object.assign('confirm' as const, { value: '', action: 'confirm' as const }),
		)
	})
	afterEach(() => { vi.restoreAllMocks(); document.body.innerHTML = '' })

	it('loads list and stats, applies filters, and changes pages', async () => {
		const wrapper = mountPage(['system:session:list'])
		await flushPromises()
		expect(getSessions).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
		expect(getSessionStats).toHaveBeenCalledOnce()
		expect(wrapper.text()).toContain('当前会话')
		expect(wrapper.text()).toContain('admin')

		await wrapper.get('[data-testid="session-username"]').setValue(' member ')
		await wrapper.get('[data-testid="session-platform"]').setValue(' portal ')
		wrapper.findComponent({ name: 'ElSelect' }).vm.$emit('update:modelValue', 'active')
		await wrapper.get('[data-testid="session-search"]').trigger('click')
		await flushPromises()
		expect(getSessions).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, username: 'member', platform: 'portal', status: 'active' })

		wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
		await flushPromises()
		expect(getSessions).toHaveBeenLastCalledWith({ page: 2, pageSize: 20, username: 'member', platform: 'portal', status: 'active' })
	})

	it('never offers the current session and reloads list and stats after revoke', async () => {
		const wrapper = mountPage(['system:session:list', 'system:session:revoke'])
		await flushPromises()
		expect(wrapper.find('[data-testid="session-revoke-1"]').exists()).toBe(false)
		await wrapper.get('[data-testid="session-revoke-2"]').trigger('click')
		await flushPromises()
		expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
		expect(revokeSession).toHaveBeenCalledWith(2)
		expect(getSessions).toHaveBeenCalledTimes(2)
		expect(getSessionStats).toHaveBeenCalledTimes(2)
	})

	it('confirms a bounded batch and reloads authoritative data', async () => {
		const wrapper = mountPage(['system:session:list', 'system:session:revoke'])
		await flushPromises()
		wrapper.findComponent({ name: 'ElTable' }).vm.$emit('selection-change', [rows()[1]])
		await nextTick()
		await wrapper.get('[data-testid="session-batch-revoke"]').trigger('click')
		await flushPromises()
		expect(revokeSessions).toHaveBeenCalledWith([2])
		expect(getSessions).toHaveBeenCalledTimes(2)
		expect(getSessionStats).toHaveBeenCalledTimes(2)
	})

	it('renders explicit loading, empty, and error states', async () => {
		let resolveList: ((value: { list: SessionItem[]; total: number; page: number; pageSize: number }) => void) | undefined
		getSessions.mockReturnValue(new Promise((resolve) => { resolveList = resolve }))
		const wrapper = mountPage(['system:session:list'])
		await nextTick()
		expect(wrapper.find('[data-testid="session-loading"]').exists()).toBe(true)
		resolveList?.({ list: [], total: 0, page: 1, pageSize: 20 })
		await flushPromises()
		expect(wrapper.text()).toContain('暂无会话')

		getSessions.mockRejectedValue(new Error('数据库不可用'))
		const failed = mountPage(['system:session:list'])
		await flushPromises()
		expect(failed.text()).toContain('数据库不可用')
	})
})

function mountPage(permissions: string[]): VueWrapper {
	const pinia = createPinia()
	setActivePinia(pinia)
	useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: permissions })
	return mount(SessionManagement, { attachTo: document.body, global: { plugins: [pinia, appI18n, ElementPlus] } })
}

function rows(): SessionItem[] {
	return [
		{ id: 1, userId: 1, username: 'admin', platform: 'admin', deviceId: 'device-current', clientIp: '127.0.0.1', userAgent: 'Chrome', createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z', refreshExpiresAt: '2026-08-22T00:00:00Z', revokedAt: null, status: 'active', isCurrent: true },
		{ id: 2, userId: 2, username: 'member', platform: 'admin', deviceId: 'device-other', clientIp: '127.0.0.2', userAgent: 'Firefox', createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z', refreshExpiresAt: '2026-08-22T00:00:00Z', revokedAt: null, status: 'active', isCurrent: false },
	]
}
