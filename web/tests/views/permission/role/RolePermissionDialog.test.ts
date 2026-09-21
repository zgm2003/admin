import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getRolePermissions } from '@/api/permission/role'
import type { RoleListItem, RolePermissionsResponse } from '@/api/permission/role'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import RolePermissionDialog from '@/views/permission/role/components/RolePermissionDialog/index.vue'

vi.mock('@/api/permission/role', () => ({
  getRolePermissions: vi.fn(),
  updateRolePermissions: vi.fn(),
}))

describe('RolePermissionDialog request lifecycle', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    document.body.innerHTML = ''
  })

  it('does not let an older role response overwrite a newly opened role', async () => {
    const older = deferred<RolePermissionsResponse>()
    vi.mocked(getRolePermissions)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce(permissionResponse(role(2, 'second', '第二角色')))
    const wrapper = mount(RolePermissionDialog, {
      attachTo: document.body,
      props: { modelValue: false, role: role(1, 'first', '第一角色') },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.setProps({ modelValue: true })
    await vi.waitFor(() => expect(getRolePermissions).toHaveBeenCalledWith(1))
    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ role: role(2, 'second', '第二角色') })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(document.body.textContent).toContain('第二角色 (second)')

    older.resolve(permissionResponse(role(1, 'first', '第一角色')))
    await flushPromises()
    expect(document.body.textContent).toContain('第二角色 (second)')
    expect(document.body.textContent).not.toContain('第一角色 (first)')

    wrapper.unmount()
  })
})

function role(id: number, code: string, name: string): RoleListItem {
  return {
    id,
    code,
    name,
    isDefault: YesNo.No,
    isEnabled: YesNo.Yes,
    userCount: 0,
    permissionCount: 0,
    createdAt: '2026-09-21T00:00:00Z',
    updatedAt: '2026-09-21T00:00:00Z',
  }
}

function permissionResponse(item: RoleListItem): RolePermissionsResponse {
  return {
    role: {
      id: item.id,
      code: item.code,
      name: item.name,
      isDefault: item.isDefault,
      isEnabled: item.isEnabled,
    },
    platforms: [],
    menuIds: [],
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}
