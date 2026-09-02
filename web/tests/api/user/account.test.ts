import { beforeEach, describe, expect, it, vi } from 'vitest'
import { YesNo } from '@/enums/yes-no'
import { request } from '@/utils/request'
import {
  deleteUser,
  getUserRoleOptions,
  getUserRoles,
  getUsers,
  updateUser,
  updateUserRoles,
  updateUserStatus,
} from '@/api/user/account'

vi.mock('@/utils/request', () => ({
  request: vi.fn(),
  ProtocolError: class ProtocolError extends Error {},
}))
const requestMock = vi.mocked(request)

describe('user API', () => {
  beforeEach(() => requestMock.mockReset())
  it('sends exact request shapes', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 2, pageSize: 50 })
    await getUsers({ page: 2, pageSize: 50, keyword: 'alice', isEnabled: YesNo.No, roleId: 7 })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/users',
      params: { page: 2, pageSize: 50, keyword: 'alice', isEnabled: YesNo.No, roleId: 7 },
    })
    requestMock.mockResolvedValueOnce({ roles: [] })
    await getUserRoleOptions()
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/users/role-options',
    })
    requestMock.mockResolvedValueOnce({
      id: 7,
      username: 'alice_new',
      phone: '+86 138-0000-0000',
      updatedAt: '2026-08-20T00:00:00Z',
    })
    await updateUser(7, { username: 'alice_new', phone: '+86 138-0000-0000' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/users/7',
      data: { username: 'alice_new', phone: '+86 138-0000-0000' },
    })
    requestMock.mockResolvedValueOnce({ id: 7, isEnabled: YesNo.No })
    await updateUserStatus(7, YesNo.No)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PATCH',
      url: '/api/admin/v1/users/7/status',
      data: { isEnabled: YesNo.No },
    })
    requestMock.mockResolvedValueOnce({})
    await deleteUser(7)
    expect(requestMock).toHaveBeenLastCalledWith({ method: 'DELETE', url: '/api/admin/v1/users/7' })
    requestMock.mockResolvedValueOnce({
      user: { id: 7, username: 'alice', email: 'a@b.com', phone: null, isEnabled: YesNo.Yes },
      roles: [{ id: 2, code: 'member', name: 'Member', isEnabled: YesNo.Yes }],
      roleIds: [2],
    })
    await getUserRoles(7)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/users/7/roles',
    })
    requestMock.mockResolvedValueOnce({ id: 7, roleCount: 2 })
    await updateUserRoles(7, { roleIds: [2, 5] })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/users/7/roles',
      data: { roleIds: [2, 5] },
    })
  })
  it('parses the backend role result into the declared DTO', async () => {
    const result = { id: 7, roleCount: 2 }
    requestMock.mockResolvedValue(result)
    await expect(updateUserRoles(7, { roleIds: [2] })).resolves.toEqual(result)
  })

  it('rejects an updated user profile without the required phone field', async () => {
    requestMock.mockResolvedValue({
      id: 7,
      username: 'alice_new',
      updatedAt: '2026-08-20T00:00:00Z',
    })

    await expect(updateUser(7, { username: 'alice_new', phone: null })).rejects.toThrow(
      'updated user profile response is invalid',
    )
  })

  it('rejects a user list item without the required phone field', async () => {
    requestMock.mockResolvedValue(
      userPage({
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        isEnabled: YesNo.Yes,
        roles: [],
        createdAt: '2026-08-20T00:00:00Z',
        updatedAt: '2026-08-20T00:00:00Z',
      }),
    )

    await expect(getUsers({ page: 1, pageSize: 20 })).rejects.toThrow(
      'user list item response is invalid',
    )
  })

  it('rejects a user list item with a non-string phone', async () => {
    requestMock.mockResolvedValue(
      userPage({
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: 1,
        isEnabled: YesNo.Yes,
        roles: [],
        createdAt: '2026-08-20T00:00:00Z',
        updatedAt: '2026-08-20T00:00:00Z',
      }),
    )

    await expect(getUsers({ page: 1, pageSize: 20 })).rejects.toThrow(
      'user list item response is invalid',
    )
  })

  it('rejects a user role summary without the required phone field', async () => {
    requestMock.mockResolvedValue({
      user: { id: 7, username: 'alice', email: 'alice@example.com', isEnabled: YesNo.Yes },
      roles: [],
      roleIds: [],
    })

    await expect(getUserRoles(7)).rejects.toThrow('user role summary response is invalid')
  })

  it('rejects a user role summary with a non-string phone', async () => {
    requestMock.mockResolvedValue({
      user: {
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: 1,
        isEnabled: YesNo.Yes,
      },
      roles: [],
      roleIds: [],
    })

    await expect(getUserRoles(7)).rejects.toThrow('user role summary response is invalid')
  })

  it('rejects an updated user profile with a non-string phone', async () => {
    requestMock.mockResolvedValue({
      id: 7,
      username: 'alice_new',
      phone: 1,
      updatedAt: '2026-08-20T00:00:00Z',
    })

    await expect(updateUser(7, { username: 'alice_new', phone: null })).rejects.toThrow(
      'updated user profile response is invalid',
    )
  })
})

function userPage(item: object): object {
  return { list: [item], total: 1, page: 1, pageSize: 20 }
}
