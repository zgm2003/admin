import { beforeEach, describe, expect, it, vi } from 'vitest'

import { YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import { request } from '../utils/request'
import {
  createRole,
  deleteRole,
  getRolePermissions,
  getRoles,
  setDefaultRole,
  updateRole,
  updateRolePermissions,
  updateRoleStatus,
} from './role'

vi.mock('../utils/request', () => ({ request: vi.fn() }))

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
      url: '/api/v1/roles',
      params: { page: 2, pageSize: 50, keyword: 'tester', isEnabled: YesNo.No },
    })
  })

  it('creates with only code and name', async () => {
    requestMock.mockResolvedValue({ id: 7 })

    await expect(createRole({ code: 'tester', name: 'Tester' })).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/roles',
      data: { code: 'tester', name: 'Tester' },
    })
  })

  it('updates with only name', async () => {
    requestMock.mockResolvedValue({})

    await expect(updateRole(7, { name: 'Updated' })).resolves.toEqual({})
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/v1/roles/7',
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
      url: '/api/v1/roles/7/status',
      data: { isEnabled: YesNo.No },
    })
  })

  it('sets default without a request body', async () => {
    requestMock.mockResolvedValue({ id: 7, isDefault: YesNo.Yes })

    await expect(setDefaultRole(7)).resolves.toEqual({ id: 7, isDefault: YesNo.Yes })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PATCH',
      url: '/api/v1/roles/7/default',
    })
  })

  it('deletes without a request body', async () => {
    requestMock.mockResolvedValue({})

    await expect(deleteRole(7)).resolves.toEqual({})
    expect(requestMock).toHaveBeenCalledWith({
      method: 'DELETE',
      url: '/api/v1/roles/7',
    })
  })

  it('gets permissions without request data', async () => {
    requestMock.mockResolvedValue(permissionResponse())

    await expect(getRolePermissions(7)).resolves.toEqual(permissionResponse())
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/v1/roles/7/permissions',
    })
  })

  it('updates permissions with only menuIds', async () => {
    requestMock.mockResolvedValue({ id: 7, permissionCount: 1 })

    await expect(updateRolePermissions(7, { menuIds: [3] })).resolves.toEqual({
      id: 7,
      permissionCount: 1,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/v1/roles/7/permissions',
      data: { menuIds: [3] },
    })
  })

  it('rejects malformed responses instead of returning compatibility data', async () => {
    requestMock.mockResolvedValue({ id: 7, permission_count: 1 })

    await expect(updateRolePermissions(7, { menuIds: [] })).rejects.toBeInstanceOf(ProtocolError)
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
    menuTree: [],
    menuIds: [],
  }
}
