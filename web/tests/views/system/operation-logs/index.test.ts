import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as operationLogAPI from '@src/api/audit/operationlog'
import type { OperationLogItem } from '@src/api/audit/operationlog'
import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import OperationLogs from '@src/views/system/operation-logs/index.vue'

vi.mock('@src/api/audit/operationlog', () => ({ getOperationLogs: vi.fn() }))
const getOperationLogs = vi.mocked(operationLogAPI.getOperationLogs)

describe('operation logs', () => {
	beforeEach(() => {
		vi.clearAllMocks()
		setLocale('zh-CN')
		getOperationLogs.mockResolvedValue({ list: [row()], total: 1, page: 1, pageSize: 20 })
	})
	afterEach(() => { document.body.innerHTML = '' })

	it('loads logs and submits exact filters and pagination', async () => {
		const wrapper = mountPage()
		await flushPromises()
		expect(getOperationLogs).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
		expect(wrapper.find('h1').exists()).toBe(false)
		expect(wrapper.get('.operation-log-page').classes()).toContain('management-page')
		await wrapper.get('[data-testid="operation-log-user-id"]').setValue('7')
		await wrapper.get('[data-testid="operation-log-action"]').setValue('user.update')
		await wrapper.get('[data-testid="operation-log-route"]').setValue('/api/v1/users')
		wrapper.findComponent({ name: 'ElSelectV2' }).vm.$emit('update:modelValue', YesNo.No)
		wrapper.findComponent({ name: 'ElDatePicker' }).vm.$emit('update:modelValue', ['2026-08-20T00:00:00+08:00', '2026-08-21T00:00:00+08:00'])
		await wrapper.get('[data-testid="operation-log-search"]').trigger('click')
		await flushPromises()
		expect(getOperationLogs).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, userId: 7, action: 'user.update', route: '/api/v1/users', isSuccess: YesNo.No, from: '2026-08-20T00:00:00+08:00', to: '2026-08-21T00:00:00+08:00' })

		wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
		await flushPromises()
		expect(getOperationLogs).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, pageSize: 20 }))
	})

	it('expands sanitized details and never renders a delete command', async () => {
		const wrapper = mountPage()
		await flushPromises()
		expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
		expect(wrapper.find('[data-testid="operation-log-delete"]').exists()).toBe(false)
		await wrapper.get('button[aria-label="Expand this row"]').trigger('click')
		await flushPromises()
		expect(wrapper.text()).toContain('***')
		expect(wrapper.text()).toContain('request-id-1')
	})

	it('renders explicit loading, empty, and error states', async () => {
		let resolveLogs: ((value: { list: OperationLogItem[]; total: number; page: number; pageSize: number }) => void) | undefined
		getOperationLogs.mockReturnValue(new Promise((resolve) => { resolveLogs = resolve }))
		const wrapper = mountPage()
		await nextTick()
		expect(wrapper.find('[data-testid="operation-log-loading"]').exists()).toBe(true)
		resolveLogs?.({ list: [], total: 0, page: 1, pageSize: 20 })
		await flushPromises()
		expect(wrapper.text()).toContain('暂无操作日志')

		getOperationLogs.mockRejectedValue(new Error('查询失败'))
		const failed = mountPage()
		await flushPromises()
		expect(failed.text()).toContain('查询失败')
	})

	it('renders a localized action name and falls back to the action code', async () => {
		const wrapper = mountPage()
		await flushPromises()
		expect(wrapper.text()).toContain('编辑用户')
		expect(wrapper.text()).toContain('admin')

		getOperationLogs.mockResolvedValue({ list: [{ ...row(), action: 'future.action' }], total: 1, page: 1, pageSize: 20 })
		const fallback = mountPage()
		await flushPromises()
		expect(fallback.text()).toContain('future.action')
	})
})

function mountPage(): VueWrapper {
	const pinia = createPinia()
	setActivePinia(pinia)
	return mount(OperationLogs, { attachTo: document.body, global: { plugins: [pinia, appI18n, ElementPlus] } })
}

function row(): OperationLogItem {
	return { id: 1, requestId: 'request-id-1', userId: 7, userName: 'admin', sessionId: 9, platform: 'admin', method: 'PUT', route: '/api/admin/v1/users/:id', module: 'user', action: 'user.update', clientIp: '127.0.0.1', userAgent: 'Chrome', statusCode: 200, isSuccess: YesNo.Yes, latencyMs: 12, requestData: { password: '***' }, responseData: { code: 0 }, createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z' }
}
