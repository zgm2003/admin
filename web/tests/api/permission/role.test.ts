import { beforeEach, describe, expect, it, vi } from 'vitest'

import { YesNo } from '@/enums/yes-no'
import { request } from '@/utils/request'
import {
  createRole,
  deleteRole,
  getRolePermissions,
  getRoles,
  setDefaultRole,
  updateRole,
  updateRolePermissions,
  updateRoleStatus,
} from '@/api/permission/role'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('role API', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('sends explicit pagination and only present filters', async () => {
    requestMock.mockResolvedValue({ list: [], total: 0, page: 2, pageSize: 50 })

    await expect(
      getRoles({ page: 2, pageSize: 50, keyword: 'tester', isEnabled: YesNo.No }),
    ).resolves.toEqual({ list: [], total: 0, page: 2, pageSize: 50 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/roles',
      params: { page: 2, pageSize: 50, keyword: 'tester', isEnabled: YesNo.No },
    })
  })

  it('creates with only code and name', async () => {
    requestMock.mockResolvedValue({ id: 7 })

    await expect(createRole({ code: 'tester', name: 'Tester' })).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/admin/v1/roles',
      data: { code: 'tester', name: 'Tester' },
    })
  })

  it('updates with only name', async () => {
    requestMock.mockResolvedValue({})

    await expect(updateRole(7, { name: 'Updated' })).resolves.toEqual({})
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/roles/7',
      data: { name: 'Updated' },
    })
  })

  it('updates status with only isEnabled', async () => {
    requestMock.mockResolvedValue({ id: 7, isEnabled: YesNo.No })

    await expect(updateRoleStatus(7, YesNo.No)).resolves.toEqual({
      id: 7,
      isEnabled: YesNo.No,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PATCH',
      url: '/api/admin/v1/roles/7/status',
      data: { isEnabled: YesNo.No },
    })
  })

  it('sets default without a request body', async () => {
    requestMock.mockResolvedValue({ id: 7, isDefault: YesNo.Yes })

    await expect(setDefaultRole(7)).resolves.toEqual({ id: 7, isDefault: YesNo.Yes })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PATCH',
      url: '/api/admin/v1/roles/7/default',
    })
  })

  it('deletes without a request body', async () => {
    requestMock.mockResolvedValue({})

    await expect(deleteRole(7)).resolves.toEqual({})
    expect(requestMock).toHaveBeenCalledWith({
      method: 'DELETE',
      url: '/api/admin/v1/roles/7',
    })
  })

  it('gets permissions without request data', async () => {
    requestMock.mockResolvedValue(permissionResponse())

    await expect(getRolePermissions(7)).resolves.toEqual(permissionResponse())
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/roles/7/permissions',
    })
  })

  it.each([
    { role: permissionResponse().role, menuTree: [], menuIds: [] },
    {
      role: permissionResponse().role,
      platforms: [{ id: 1, code: 'admin', name: 'Admin', menuTree: [] }],
      menuIds: [],
    },
    { role: permissionResponse().role, platforms: [], menuIds: [], extra: true },
  ])('rejects invalid permission responses: %j', async (value) => {
    requestMock.mockResolvedValue(value)
    await expect(getRolePermissions(7)).rejects.toThrow('role permissions response is invalid')
  })

  it('updates permissions with only menuIds', async () => {
    requestMock.mockResolvedValue({ id: 7, permissionCount: 1 })

    await expect(updateRolePermissions(7, { menuIds: [3] })).resolves.toEqual({
      id: 7,
      permissionCount: 1,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/roles/7/permissions',
      data: { menuIds: [3] },
    })
  })

  it('rejects legacy result field names', async () => {
    const result = { id: 7, permission_count: 1 }
    requestMock.mockResolvedValue(result)
    await expect(updateRolePermissions(7, { menuIds: [] })).rejects.toThrow(
      'role permission count must be an integer',
    )
  })
})

function permissionResponse() {
  return {
    role: {
      id: 7,
      code: 'tester',
      name: 'Tester',
      isDefault: YesNo.No,
      isEnabled: YesNo.Yes,
    },
    platforms: [
      {
        id: 1,
        code: 'admin',
        name: 'Admin',
        isEnabled: YesNo.Yes,
        menuTree: [
          {
            id: 3,
            parentId: null,
            menuType: 'page',
            code: 'admin:test',
            name: 'Admin Test',
            isEnabled: YesNo.Yes,
            children: [],
          },
        ],
      },
      {
        id: 2,
        code: 'canvas',
        name: 'Canvas',
        isEnabled: YesNo.No,
        menuTree: [
          {
            id: 20,
            parentId: null,
            menuType: 'page',
            code: 'canvas:test:list',
            name: 'Canvas Test',
            isEnabled: YesNo.Yes,
            children: [],
          },
        ],
      },
    ],
    menuIds: [3, 20],
  }
}
