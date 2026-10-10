// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/user/profile'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getAccountProfile preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getAccountProfile()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/profile')
  })
  it('getAccountProfile propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getAccountProfile()).rejects.toBe(error)
  })
  it('updateAccountProfile preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.updateAccountProfile>[0] = {
      username: 'sample',
      avatar: 'sample',
      birthday: null,
      gender: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateAccountProfile(input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/profile', input)
  })
  it('updateAccountProfile propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.updateAccountProfile>[0] = {
      username: 'sample',
      avatar: 'sample',
      birthday: null,
      gender: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateAccountProfile(input)).rejects.toBe(error)
  })
  it('changePassword preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.changePassword>[0] = {
      currentPassword: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.changePassword(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/password', input)
  })
  it('changePassword propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.changePassword>[0] = {
      currentPassword: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.changePassword(input)).rejects.toBe(error)
  })
  it('setPassword preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.setPassword>[0] = {
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.setPassword(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/password/set', input)
  })
  it('setPassword propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.setPassword>[0] = {
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.setPassword(input)).rejects.toBe(error)
  })
  it('sendPasswordCode preserves the HTTP contract and backend data', async () => {
    const loginType: Parameters<typeof api.sendPasswordCode>[0] = 'email'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendPasswordCode(loginType)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/password/send-code', {
      loginType,
    })
  })
  it('sendPasswordCode propagates request failures unchanged', async () => {
    const loginType: Parameters<typeof api.sendPasswordCode>[0] = 'email'
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendPasswordCode(loginType)).rejects.toBe(error)
  })
  it('changePasswordByCode preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.changePasswordByCode>[0] = {
      loginType: 'email',
      challengeId: 'sample',
      code: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.changePasswordByCode(input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/user/password/by-code',
      input,
    )
  })
  it('changePasswordByCode propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.changePasswordByCode>[0] = {
      loginType: 'email',
      challengeId: 'sample',
      code: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.changePasswordByCode(input)).rejects.toBe(error)
  })
})
