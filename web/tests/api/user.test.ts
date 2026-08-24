import { beforeEach, describe, expect, it, vi } from 'vitest'
import { YesNo } from '@src/enums/yes-no'
import { ProtocolError } from '@src/types/http'
import { request } from '@src/utils/request'
import { deleteUser, getUserRoleOptions, getUserRoles, getUsers, updateUser, updateUserRoles, updateUserStatus } from '@src/api/user'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('user API', () => {
  beforeEach(() => requestMock.mockReset())
  it('sends exact request shapes', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 2, pageSize: 50 })
    await getUsers({ page: 2, pageSize: 50, keyword: 'alice', isEnabled: YesNo.No, roleId: 7 })
    expect(requestMock).toHaveBeenLastCalledWith({ method:'GET', url:'/api/v1/users', params:{ page:2, pageSize:50, keyword:'alice', isEnabled:YesNo.No, roleId:7 } })
    requestMock.mockResolvedValueOnce({ roles: [] }); await getUserRoleOptions(); expect(requestMock).toHaveBeenLastCalledWith({ method:'GET', url:'/api/v1/users/role-options' })
    requestMock.mockResolvedValueOnce({ id:7, username:'alice_new', updatedAt:'2026-08-20T00:00:00Z' }); await updateUser(7,{ username:'alice_new' }); expect(requestMock).toHaveBeenLastCalledWith({ method:'PUT', url:'/api/v1/users/7', data:{ username:'alice_new' } })
    requestMock.mockResolvedValueOnce({ id:7, isEnabled:YesNo.No }); await updateUserStatus(7,YesNo.No); expect(requestMock).toHaveBeenLastCalledWith({ method:'PATCH', url:'/api/v1/users/7/status', data:{ isEnabled:YesNo.No } })
    requestMock.mockResolvedValueOnce({}); await deleteUser(7); expect(requestMock).toHaveBeenLastCalledWith({ method:'DELETE', url:'/api/v1/users/7' })
    requestMock.mockResolvedValueOnce({ user:{id:7,username:'alice',email:'a@b.com',isEnabled:YesNo.Yes}, roles:[{id:2,code:'member',name:'Member',isEnabled:YesNo.Yes}], roleIds:[2] }); await getUserRoles(7); expect(requestMock).toHaveBeenLastCalledWith({ method:'GET', url:'/api/v1/users/7/roles' })
    requestMock.mockResolvedValueOnce({ id:7, roleCount:2 }); await updateUserRoles(7,{roleIds:[2,5]}); expect(requestMock).toHaveBeenLastCalledWith({ method:'PUT', url:'/api/v1/users/7/roles', data:{roleIds:[2,5]} })
  })
  it('rejects malformed responses', async () => {
    requestMock.mockResolvedValue({ id: 7, role_count: 2 })
    await expect(updateUserRoles(7, { roleIds: [2] })).rejects.toBeInstanceOf(ProtocolError)
  })
})
