import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProfilePage } from '@/views/user/profile'
import { authStore, permissionStore, request } from '@/store'

const profile = { userId: 7, username: 'Canvas user', email: 'user@example.com', phone: null, avatar: '', birthday: null, gender: 0 }
const codes = ['user:profile:view', 'user:profile:detail', 'user:profile:update', 'user:password:update', 'user:email:update', 'user:phone:update']
const mount = () => render(<MemoryRouter><ProfilePage /></MemoryRouter>)
beforeEach(async () => {
  authStore.setAuthenticated({ accessToken: 'token', expiresIn: 3600, isNewUser: false, passwordSetRequired: false }, { ...profile, passwordSetRequired: false })
  permissionStore.reset()
  vi.spyOn(request, 'get').mockImplementation(async path => {
    if (path === '/api/canvas/v1/access') return { roleCodes: [], permissionCodes: codes, menuTree: [{ code: 'user:profile:view', menuType: 'page', path: '/user/profile', componentPath: 'user/profile', i18nKey: 'layout.user.profile', icon: null, isHidden: 1, children: [] }] }
    if (path.endsWith('/options')) return { genders: [{ value: 0, label: '未设置' }] }
    if (path === '/api/canvas/v1/auth/me') return authStore.state.user
    return profile
  })
  await permissionStore.load()
})

describe('Personal center security and partial failures', () => {
  it.each([
    { name: 'leaving the page', unmount: true, userId: 7, accessToken: 'token' },
    { name: 'logging into another account after leaving the page', unmount: true, userId: 99, accessToken: 'new-session' },
    { name: 'switching accounts while the component remains mounted', unmount: false, userId: 99, accessToken: 'new-session' },
    { name: 'starting a new session for the same account', unmount: false, userId: 7, accessToken: 'new-session' },
  ])('never clears the current session when an old password request completes after $name', async ({ unmount, userId, accessToken }) => {
    let complete!: () => void
    vi.spyOn(request, 'post').mockImplementation(() => new Promise<void>(resolve => { complete = resolve }))
    const view = mount(); await screen.findByLabelText('用户名')
    fireEvent.change(screen.getByLabelText('当前密码'), { target: { value: 'OldPassword123!' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'NewPassword123!' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'NewPassword123!' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(request.post).toHaveBeenCalledWith('/api/canvas/v1/user/password', expect.anything()))
    if (unmount) view.unmount()
    authStore.setAuthenticated({ accessToken, expiresIn: 3600, isNewUser: false, passwordSetRequired: false }, { ...profile, userId, passwordSetRequired: false })
    await act(async () => { complete(); await Promise.resolve() })
    expect(authStore.state.status).toBe('authenticated'); expect(authStore.state.user?.userId).toBe(userId); expect(request.getAccessToken()).toBe(accessToken)
  })
  it('clears the original session and redirects to login after a successful password change', async () => {
    vi.spyOn(request, 'post').mockResolvedValue({})
    render(<MemoryRouter initialEntries={['/user/profile']}><Routes><Route path="/user/profile" element={<ProfilePage />} /><Route path="/login" element={<p>登录页面</p>} /></Routes></MemoryRouter>)
    await screen.findByLabelText('用户名')
    fireEvent.change(screen.getByLabelText('当前密码'), { target: { value: 'OldPassword123!' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'NewPassword123!' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'NewPassword123!' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    expect(await screen.findByText('登录页面')).toBeInTheDocument()
    expect(authStore.state.status).toBe('anonymous'); expect(request.getAccessToken()).toBe('')
  })
  it('requires both proofs when changing an existing email', async () => {
    vi.spyOn(request, 'post').mockResolvedValue({ challengeId: 'proof', expiresAt: '2030-01-01', resendAfterSeconds: 60 })
    const put = vi.spyOn(request, 'put').mockResolvedValue({ email: 'next@example.com' })
    mount(); await screen.findByLabelText('用户名'); fireEvent.click(screen.getByRole('button', { name: '更换邮箱' }))
    fireEvent.click(screen.getByRole('button', { name: '发送当前邮箱验证码' }))
    await waitFor(() => expect(request.post).toHaveBeenCalledWith('/api/canvas/v1/user/email/send-code', { target: 'current' }))
    fireEvent.change(screen.getByLabelText('当前邮箱验证码'), { target: { value: '123456' } })
    fireEvent.change(screen.getByLabelText('新邮箱'), { target: { value: 'next@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: '发送新邮箱验证码' }))
    await waitFor(() => expect(request.post).toHaveBeenCalledWith('/api/canvas/v1/user/email/send-code', { target: 'next', email: 'next@example.com', challengeId: undefined }))
    fireEvent.change(screen.getByLabelText('新邮箱验证码'), { target: { value: '654321' } })
    fireEvent.click(screen.getByRole('button', { name: '确认更换' }))
    await waitFor(() => expect(put).toHaveBeenCalledWith('/api/canvas/v1/user/email', { nextEmail: 'next@example.com', nextChallengeId: 'proof', nextCode: '654321', currentChallengeId: 'proof', currentCode: '123456' }))
    expect(await screen.findByText('next@example.com')).toBeInTheDocument()
  })
  it('keeps a successful save visible if refreshing identity fails', async () => {
    vi.spyOn(request, 'put').mockResolvedValue({ ...profile, username: 'New name', updatedAt: 'now' })
    vi.spyOn(authStore, 'reloadUser').mockRejectedValue(new Error('刷新暂时失败'))
    mount(); fireEvent.change(await screen.findByLabelText('用户名'), { target: { value: 'New name' } }); fireEvent.click(screen.getByRole('button', { name: '保存资料' }))
    expect(await screen.findByText('资料已保存')).toBeInTheDocument(); expect(await screen.findByText(/刷新暂时失败/)).toBeInTheDocument()
    expect(screen.getByLabelText('用户名')).toHaveValue('New name')
  })
  it('changes password by verification code and keeps the current session', async () => {
    vi.spyOn(request, 'post').mockResolvedValue({ challengeId: 'password-proof', expiresAt: '2030-01-01', resendAfterSeconds: 60 })
    const put = vi.spyOn(request, 'put').mockResolvedValue({})
    mount(); await screen.findByLabelText('用户名'); fireEvent.click(screen.getByRole('button', { name: '验证码修改' }))
    expect(screen.queryByLabelText('当前密码')).not.toBeInTheDocument(); fireEvent.click(screen.getByRole('button', { name: '发送密码验证码' }))
    await waitFor(() => expect(request.post).toHaveBeenCalledWith('/api/canvas/v1/user/password/send-code', { loginType: 'email' }))
    fireEvent.change(screen.getByLabelText('密码验证码'), { target: { value: '123456' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'Password123!' } }); fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'Password123!' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(put).toHaveBeenCalledWith('/api/canvas/v1/user/password/by-code', { loginType: 'email', challengeId: 'password-proof', code: '123456', newPassword: 'Password123!', confirmPassword: 'Password123!' }))
    expect(await screen.findByText('密码已修改')).toBeInTheDocument(); expect(authStore.state.status).toBe('authenticated')
  })
  it('retains an avatar adoption key on retry and clears it after success', async () => {
    permissionStore.state = { ...permissionStore.state, permissionCodes: [...codes, 'storage:object:upload', 'storage:file:detail'] }
    const originalCreate = Object.getOwnPropertyDescriptor(URL, 'createObjectURL')
    const originalRevoke = Object.getOwnPropertyDescriptor(URL, 'revokeObjectURL')
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: () => 'blob:avatar-preview' })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: () => undefined })
    try {
      vi.spyOn(request, 'post').mockImplementation(async path => path.endsWith('/upload-credential') ? { items: [{ fileId: 9, method: 'PUT', uploadUrl: 'https://storage.invalid/signed', objectKey: 'file/test.png', headers: {}, expiresAt: '2030-01-01' }] } : path.endsWith('/complete') ? { fileId: 9, objectKey: 'file/test.png' } : { url: 'https://storage.invalid/avatar' })
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('', { status: 200 }))
      const put = vi.spyOn(request, 'put').mockRejectedValueOnce(new Error('暂时失败')).mockResolvedValue({ ...profile, avatar: 'avatar/saved.png', updatedAt: 'now' })
      mount(); await screen.findByLabelText('用户名'); fireEvent.change(screen.getByLabelText('头像文件'), { target: { files: [new File(['png'], 'image.png', { type: 'image/png' })] } })
      await screen.findByText('头像已上传，保存资料后生效')
      fireEvent.click(screen.getByRole('button', { name: '保存资料' })); await screen.findByText('暂时失败')
      fireEvent.click(screen.getByRole('button', { name: '保存资料' })); await screen.findByText('资料已保存')
      expect(put.mock.calls[0][1]).toMatchObject({ avatar: '', avatarFileId: 9, avatarRequestKey: expect.any(String) }); expect(put.mock.calls[1][1]).toEqual(put.mock.calls[0][1])
      expect(screen.queryByText('头像已上传，保存资料后生效')).not.toBeInTheDocument()
    } finally {
      if (originalCreate) Object.defineProperty(URL, 'createObjectURL', originalCreate); else Reflect.deleteProperty(URL, 'createObjectURL')
      if (originalRevoke) Object.defineProperty(URL, 'revokeObjectURL', originalRevoke); else Reflect.deleteProperty(URL, 'revokeObjectURL')
    }
  })
})
