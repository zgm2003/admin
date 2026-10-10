import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import * as userAPI from '@/api/user/account'
import * as emailAPI from '@/api/user/email'
import * as phoneAPI from '@/api/user/phone'
import UserManagement from '@/views/user/account/index.vue'
import UserRoleDialog from '@/views/user/account/components/UserRoleDialog/index.vue'

vi.mock('@/api/user/account', () => ({
  getUsers: vi.fn(),
  getUserFormOptions: vi.fn(),
  getUserRoleOptions: vi.fn(),
  updateUser: vi.fn(),
  updateUserStatus: vi.fn(),
  deleteUser: vi.fn(),
  getUserRoles: vi.fn(),
  updateUserRoles: vi.fn(),
}))
vi.mock('@/api/user/email', () => ({ getEmailChangeLogs: vi.fn() }))
vi.mock('@/api/user/phone', () => ({ getPhoneChangeLogs: vi.fn() }))
const getUsers = vi.mocked(userAPI.getUsers)
const getRoleOptions = vi.mocked(userAPI.getUserRoleOptions)
const updateUser = vi.mocked(userAPI.updateUser)
const updateStatus = vi.mocked(userAPI.updateUserStatus)
const deleteUser = vi.mocked(userAPI.deleteUser)
const getUserRoles = vi.mocked(userAPI.getUserRoles)
const updateRoles = vi.mocked(userAPI.updateUserRoles)
const getEmailChangeLogs = vi.mocked(emailAPI.getEmailChangeLogs)
const getPhoneChangeLogs = vi.mocked(phoneAPI.getPhoneChangeLogs)

describe('user management', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getUserRoles.mockReset()
    updateRoles.mockReset()
    vi.mocked(userAPI.getUserFormOptions).mockResolvedValue({
      usernameMinLength: 3,
      usernameMaxLength: 64,
      usernamePattern: String.raw`^[\p{L}\p{Nd}_-]+$`,
    })
    setLocale('zh-CN')
    getRoleOptions.mockResolvedValue({ roles: roles() })
    getUsers.mockResolvedValue({ list: [row()], total: 1, page: 1, pageSize: 20 })
    updateUser.mockResolvedValue({
      id: 7,
      username: 'new_name',
      phone: '+86 138-0000-0000',
      updatedAt: '2026-08-20T02:00:00Z',
    })
    updateStatus.mockResolvedValue({ id: 7, isEnabled: YesNo.No })
    deleteUser.mockResolvedValue({})
    getUserRoles.mockResolvedValue({
      user: {
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: '+86 138-0000-0000',
        isEnabled: YesNo.Yes,
      },
      roles: roles(),
      roleIds: [2, 3],
    })
    updateRoles.mockResolvedValue({ id: 7, roleCount: 2 })
    getEmailChangeLogs.mockResolvedValue({
      list: [
        {
          id: 1,
          action: 1,
          actionLabel: '修改邮箱',
          oldEmail: 'old@example.com',
          newEmail: 'new@example.com',
          platform: 'admin',
          createdAt: '2026-09-29T05:00:00Z',
        },
        {
          id: 2,
          action: 2,
          actionLabel: '首次绑定',
          oldEmail: null,
          newEmail: 'bound@example.com',
          platform: 'admin',
          createdAt: '2026-09-28T05:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    getPhoneChangeLogs.mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
    vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () =>
      Object.assign('confirm' as const, { value: '', action: 'confirm' as const }),
    )
  })
  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('loads, renders roles, and applies filters and paging', async () => {
    const wrapper = mountPage(['user:account:list'])
    await flushPromises()
    expect(getRoleOptions).toHaveBeenCalledTimes(1)
    expect(getUsers).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.user-management').classes()).toContain('management-page')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('+86 138-0000-0000')
    expect(wrapper.text()).toContain('角色已禁用')
    expect(wrapper.get('[data-testid="user-keyword"]').attributes('placeholder')).toBe(
      '用户名、邮箱或手机号',
    )
    setLocale('en-US')
    await flushPromises()
    expect(wrapper.get('[data-testid="user-keyword"]').attributes('placeholder')).toBe(
      'Username, email, or phone',
    )
    setLocale('zh-CN')
    await flushPromises()
    await wrapper.get('.user-filters input').setValue(' alice ')
    const selects = wrapper.findAllComponents({ name: 'ElSelectV2' })
    selects[0].vm.$emit('update:modelValue', YesNo.No)
    selects[1].vm.$emit('update:modelValue', 2)
    await findButton(wrapper, '查询').trigger('click')
    await flushPromises()
    expect(getUsers).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: 'alice',
      isEnabled: YesNo.No,
      roleId: 2,
    })
  })

  it('consumes backend action decisions without inferring actor or role protection', async () => {
    getUsers.mockResolvedValueOnce({
      list: [
        {
          ...row(),
          actions: { update: false, status: false, delete: false, authorize: false },
          actionLabels: {
            update: '后端编辑保护',
            status: '后端状态保护',
            delete: '后端删除保护',
            authorize: '后端授权保护',
          },
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(
      [
        'user:account:update',
        'user:account:status',
        'user:account:delete',
        'user:account:authorize',
      ],
      9,
    )
    await flushPromises()
    expect(findAriaButton(wrapper, '编辑').attributes('disabled')).toBeDefined()
    expect(findAriaButton(wrapper, '分配角色').attributes('disabled')).toBeDefined()
    expect(updateUser).not.toHaveBeenCalled()
  })

  it('formats user timestamps instead of rendering raw API strings', async () => {
    const wrapper = mountPage(['user:account:list'])
    await flushPromises()

    expect(wrapper.text()).toContain('2026年8月20日')
    expect(wrapper.text()).not.toContain('2026-08-20T00:00:00')
    expect(wrapper.text()).not.toContain('2026-08-20T01:00:00')
  })

  it('renders only granted commands and consumes backend self protection', async () => {
    getUsers.mockResolvedValueOnce({
      list: [
        { ...row(), actions: { update: true, status: false, delete: false, authorize: false } },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage([
      'user:account:update',
      'user:account:status',
      'user:account:delete',
      'user:account:authorize',
    ])
    await flushPromises()
    const texts = wrapper
      .findAll('button')
      .map((button) => button.attributes('aria-label') ?? button.text())
    expect(texts.join(' ')).toContain('编辑')
    expect(texts.join(' ')).toContain('分配角色')
    const dangerous = wrapper
      .findAll('button')
      .filter((button) => ['已禁用', '删除用户', '分配角色'].includes(button.text()))
    expect(dangerous.every((button) => button.attributes('disabled') !== undefined)).toBe(true)
  })

  it('shows email history only with detail action and renders plaintext values', async () => {
    const withoutDetail = mountPage(['user:account:list'])
    await flushPromises()
    expect(withoutDetail.text()).not.toContain('身份变更记录')

    const wrapper = mountPage(['user:account:list', 'user:account:detail'])
    await flushPromises()
    await findAriaButton(wrapper, '身份变更记录').trigger('click')
    await flushPromises()
    expect(getEmailChangeLogs).toHaveBeenCalledWith(7, { page: 1, pageSize: 20 })
    expect(getPhoneChangeLogs).toHaveBeenCalledWith(7, { page: 1, pageSize: 20 })
    expect(document.body.textContent).toContain('身份变更记录')
    expect(document.body.textContent).toContain('old@example.com')
    expect(document.body.textContent).toContain('new@example.com')
    expect(document.body.textContent).toContain('首次绑定')
  })

  it('edits only username while keeping email and phone as read-only identities', async () => {
    const wrapper = mountPage(['user:account:update'])
    await flushPromises()
    const access = usePermissionStore()
    const loadAccess = vi.spyOn(access, 'load')
    await findAriaButton(wrapper, '编辑').trigger('click')
    await flushPromises()
    const usernameInput = document.body.querySelector<HTMLInputElement>(
      '.user-edit-dialog input:not([disabled])',
    )
    expect(usernameInput).not.toBeNull()
    if (usernameInput === null) return
    usernameInput.value = ' new_name '
    usernameInput.dispatchEvent(new Event('input'))
    const emailInput = document.body.querySelector<HTMLInputElement>(
      '[data-testid="user-email-readonly"]',
    )
    const phoneInput = document.body.querySelector<HTMLInputElement>(
      '[data-testid="user-phone-readonly"]',
    )
    expect(emailInput?.disabled).toBe(true)
    expect(emailInput?.value).toBe('alice@example.com')
    expect(phoneInput?.disabled).toBe(true)
    expect(phoneInput?.value).toBe('+86 138-0000-0000')
    await bodyButton('保存').trigger('click')
    await flushPromises()
    expect(updateUser).toHaveBeenCalledWith(7, { username: 'new_name' })
    expect(useAuthStore().user?.username).toBe('new_name')
    expect(useAuthStore().user?.email).toBe('alice@example.com')
    expect(useAuthStore().user?.phone).toBe('+86 138-0000-0000')
    expect(loadAccess).not.toHaveBeenCalled()
  })

  it('loads and saves unique sorted user roles without refreshing access', async () => {
    const wrapper = mountPage(['user:account:authorize'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '分配角色').trigger('click')
    await flushPromises()
    expect(getUserRoles).toHaveBeenCalledWith(7)
    await bodyButton('全选').trigger('click')
    await bodyButton('保存').trigger('click')
    await flushPromises()
    expect(updateRoles).toHaveBeenCalledWith(7, { roleIds: [2, 3] })
    expect(vi.spyOn(usePermissionStore(), 'load')).not.toHaveBeenCalled()
  })

  describe('role assignment lifecycle', () => {
    beforeEach(() => {
      getUsers.mockResolvedValue({
        list: [row(), { ...row(), id: 8, username: 'bob' }],
        total: 2,
        page: 1,
        pageSize: 20,
      })
    })
    it('keeps B ownership when A succeeds after B has loaded', async () => {
      const a = deferred<userAPI.UserRolesResponse>()
      const b = deferred<userAPI.UserRolesResponse>()
      getUserRoles.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
      const wrapper = mountPage(['user:account:authorize'], 9)
      await flushPromises()
      const buttons = wrapper.findAll('button').filter((button) => button.text() === '分配角色')
      await buttons[0].trigger('click')
      await buttons[1].trigger('click')
      b.resolve(roleResponse(8, [3]))
      await flushPromises()
      a.resolve(roleResponse(7, [2]))
      await flushPromises()
      const dialog = wrapper.getComponent(UserRoleDialog)
      expect(dialog.props('roleData')?.user.id).toBe(8)
      dialog.vm.$emit('save')
      await flushPromises()
      expect(updateRoles).toHaveBeenCalledExactlyOnceWith(8, { roleIds: [3] })
      wrapper.unmount()
    })
    it('invalidates an old request when reopening the same user', async () => {
      const oldRequest = deferred<userAPI.UserRolesResponse>()
      const newRequest = deferred<userAPI.UserRolesResponse>()
      getUserRoles.mockReturnValueOnce(oldRequest.promise).mockReturnValueOnce(newRequest.promise)
      const wrapper = mountPage(['user:account:authorize'], 9)
      await flushPromises()
      await findAriaButton(wrapper, '分配角色').trigger('click')
      const dialog = wrapper.getComponent(UserRoleDialog)
      dialog.vm.$emit('update:modelValue', false)
      await flushPromises()
      await findAriaButton(wrapper, '分配角色').trigger('click')
      newRequest.resolve(roleResponse(7, [3]))
      await flushPromises()
      oldRequest.resolve(roleResponse(7, [2]))
      await flushPromises()
      expect(dialog.props('selectedRoleIDs')).toEqual([3])
      dialog.vm.$emit('save')
      await flushPromises()
      expect(updateRoles).toHaveBeenCalledExactlyOnceWith(7, { roleIds: [3] })
      wrapper.unmount()
    })
    it.each(['success', 'failure'] as const)(
      'ignores A %s while B is still loading',
      async (outcome) => {
        const a = deferred<userAPI.UserRolesResponse>()
        const b = deferred<userAPI.UserRolesResponse>()
        getUserRoles.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
        const wrapper = mountPage(['user:account:authorize'], 9)
        await flushPromises()
        const buttons = wrapper.findAll('button').filter((button) => button.text() === '分配角色')
        await buttons[0].trigger('click')
        await buttons[1].trigger('click')
        if (outcome === 'success') a.resolve(roleResponse(7, [2]))
        else a.reject(new Error('stale A load failure'))
        await flushPromises()
        const dialog = wrapper.getComponent(UserRoleDialog)
        expect(dialog.props('roleLoading')).toBe(true)
        expect(dialog.props('roleError')).toBe('')
        expect(dialog.props('roleData')).toBeNull()
        b.resolve(roleResponse(8, [3]))
        await flushPromises()
        expect(dialog.props('roleData')?.user.id).toBe(8)
        wrapper.unmount()
      },
    )
    it.each(['success', 'failure'] as const)(
      'invalidates a closed dialog before a late %s',
      async (outcome) => {
        const a = deferred<userAPI.UserRolesResponse>()
        getUserRoles.mockReturnValueOnce(a.promise)
        const wrapper = mountPage(['user:account:authorize'], 9)
        await flushPromises()
        await findAriaButton(wrapper, '分配角色').trigger('click')
        const dialog = wrapper.getComponent(UserRoleDialog)
        dialog.vm.$emit('update:modelValue', false)
        await flushPromises()
        if (outcome === 'success') a.resolve(roleResponse(7, [2]))
        else a.reject(new Error('closed dialog failure'))
        await flushPromises()
        expect(dialog.props('modelValue')).toBe(false)
        expect(dialog.props('roleData')).toBeNull()
        expect(dialog.props('selectedRoleIDs')).toEqual([])
        expect(dialog.props('roleError')).toBe('')
        dialog.vm.$emit('save')
        await flushPromises()
        expect(updateRoles).not.toHaveBeenCalled()
        wrapper.unmount()
      },
    )
    it('invalidates in-flight roles when authorize permission is lost', async () => {
      const a = deferred<userAPI.UserRolesResponse>()
      getUserRoles.mockReturnValueOnce(a.promise)
      const wrapper = mountPage(['user:account:authorize'], 9)
      await flushPromises()
      await findAriaButton(wrapper, '分配角色').trigger('click')
      usePermissionStore().permissionCodes = []
      await flushPromises()
      const dialog = wrapper.getComponent(UserRoleDialog)
      expect(dialog.props('modelValue')).toBe(false)
      a.resolve(roleResponse(7, [2]))
      await flushPromises()
      expect(dialog.props('roleData')).toBeNull()
      expect(dialog.props('selectedRoleIDs')).toEqual([])
      dialog.vm.$emit('save')
      await flushPromises()
      expect(updateRoles).not.toHaveBeenCalled()
      wrapper.unmount()
    })
    it('never saves selections whose response belongs to a different user', async () => {
      getUserRoles.mockResolvedValueOnce(roleResponse(8, [3]))
      const wrapper = mountPage(['user:account:authorize'], 9)
      await flushPromises()
      await findAriaButton(wrapper, '分配角色').trigger('click')
      await flushPromises()
      wrapper.getComponent(UserRoleDialog).vm.$emit('save')
      await flushPromises()
      expect(updateRoles).not.toHaveBeenCalled()
      wrapper.unmount()
    })
    it('does not revive loaded selections when permission is restored', async () => {
      getUserRoles.mockResolvedValueOnce(roleResponse(7, [2]))
      const wrapper = mountPage(['user:account:authorize'], 9)
      await flushPromises()
      await findAriaButton(wrapper, '分配角色').trigger('click')
      await flushPromises()
      const dialog = wrapper.getComponent(UserRoleDialog)
      usePermissionStore().permissionCodes = []
      await flushPromises()
      usePermissionStore().permissionCodes = ['user:account:authorize']
      await flushPromises()
      expect(dialog.props('modelValue')).toBe(false)
      expect(dialog.props('roleData')).toBeNull()
      expect(dialog.props('selectedRoleIDs')).toEqual([])
      dialog.vm.$emit('save')
      await flushPromises()
      expect(updateRoles).not.toHaveBeenCalled()
      wrapper.unmount()
    })
    it.each(['success', 'failure'] as const)(
      'locks writes and does not let A save %s mutate reopened B',
      async (outcome) => {
        const write = deferred<userAPI.UserRoleResult>()
        updateRoles.mockReturnValueOnce(write.promise)
        getUserRoles
          .mockResolvedValueOnce(roleResponse(7, [2]))
          .mockResolvedValueOnce(roleResponse(8, [3]))
        const wrapper = mountPage(['user:account:authorize'], 9)
        await flushPromises()
        const buttons = wrapper.findAll('button').filter((button) => button.text() === '分配角色')
        await buttons[0].trigger('click')
        await flushPromises()
        const dialog = wrapper.getComponent(UserRoleDialog)
        dialog.vm.$emit('save')
        await flushPromises()
        expect(updateRoles).toHaveBeenCalledExactlyOnceWith(7, { roleIds: [2] })
        expect(dialog.getComponent({ name: 'ElCheckboxGroup' }).props('disabled')).toBe(true)
        const toolbar = Array.from(
          document.body.querySelectorAll<HTMLButtonElement>('.user-role-dialog button'),
        ).filter((button) => ['全选', '清空'].includes(button.textContent?.trim() ?? ''))
        expect(toolbar).toHaveLength(2)
        expect(toolbar.every((button) => button.disabled)).toBe(true)
        dialog.vm.$emit('update:selectedRoleIDs', [3])
        dialog.vm.$emit('select-all')
        dialog.vm.$emit('clear')
        dialog.vm.$emit('save')
        await flushPromises()
        expect(dialog.props('selectedRoleIDs')).toEqual([2])
        expect(updateRoles).toHaveBeenCalledTimes(1)
        dialog.vm.$emit('update:modelValue', false)
        await flushPromises()
        await buttons[1].trigger('click')
        await flushPromises()
        dialog.vm.$emit('save')
        await flushPromises()
        expect(updateRoles).toHaveBeenCalledTimes(1)
        if (outcome === 'success') write.resolve({ id: 7, roleCount: 1 })
        else write.reject(new Error('stale A save failure'))
        await flushPromises()
        expect(dialog.props('modelValue')).toBe(true)
        expect(dialog.props('roleData')?.user.id).toBe(8)
        expect(dialog.props('roleError')).toBe('')
        expect(dialog.props('roleSaving')).toBe(false)
        dialog.vm.$emit('save')
        await flushPromises()
        expect(updateRoles).toHaveBeenLastCalledWith(8, { roleIds: [3] })
        wrapper.unmount()
      },
    )
    it.each(['success', 'failure'] as const)(
      'does not refresh the page after an unmounted save %s',
      async (outcome) => {
        const write = deferred<userAPI.UserRoleResult>()
        updateRoles.mockReturnValueOnce(write.promise)
        getUserRoles.mockResolvedValueOnce(roleResponse(7, [2]))
        const wrapper = mountPage(['user:account:authorize'], 9)
        await flushPromises()
        await findAriaButton(wrapper, '分配角色').trigger('click')
        await flushPromises()
        wrapper.getComponent(UserRoleDialog).vm.$emit('save')
        await flushPromises()
        wrapper.unmount()
        if (outcome === 'success') write.resolve({ id: 7, roleCount: 1 })
        else write.reject(new Error('unmounted save failure'))
        await flushPromises()
        expect(getUsers).toHaveBeenCalledTimes(1)
        expect(updateRoles).toHaveBeenCalledExactlyOnceWith(7, { roleIds: [2] })
      },
    )
  })

  it('does not change the super administrator selection when an ordinary actor selects all roles', async () => {
    getUserRoles.mockResolvedValue({
      user: {
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: '+86 138-0000-0000',
        isEnabled: YesNo.Yes,
      },
      roles: [
        {
          id: 3,
          code: 'ai_tester',
          name: 'AI Tester',
          isEnabled: YesNo.No,
          selectable: true,
          locked: false,
        },
        {
          id: 2,
          code: 'member',
          name: 'Member',
          isEnabled: YesNo.Yes,
          selectable: true,
          locked: false,
        },
        {
          id: 1,
          code: 'super_admin',
          name: 'Super Admin',
          isEnabled: YesNo.Yes,
          selectable: false,
          locked: false,
        },
      ],
      roleIds: [2],
    })
    const wrapper = mountPage(['user:account:authorize'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '分配角色').trigger('click')
    await flushPromises()
    await bodyButton('全选').trigger('click')
    await bodyButton('保存').trigger('click')
    await flushPromises()
    expect(updateRoles).toHaveBeenCalledWith(7, { roleIds: [2, 3] })
  })

  it('keeps backend-locked selections on clear and allows backend to reject an empty selection', async () => {
    getUserRoles.mockResolvedValueOnce({
      user: {
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: null,
        isEnabled: YesNo.Yes,
      },
      roles: [
        {
          id: 1,
          code: 'backend_locked',
          name: 'Locked',
          isEnabled: YesNo.No,
          selectable: false,
          locked: true,
        },
        ...roles(),
      ],
      roleIds: [1, 2],
    })
    const wrapper = mountPage(['user:account:authorize'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '分配角色').trigger('click')
    await flushPromises()
    await bodyButton('清空').trigger('click')
    await bodyButton('保存').trigger('click')
    await flushPromises()
    expect(updateRoles).toHaveBeenCalledWith(7, { roleIds: [1] })
    getUserRoles.mockResolvedValueOnce({
      user: {
        id: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: null,
        isEnabled: YesNo.Yes,
      },
      roles: roles(),
      roleIds: [2],
    })
    updateRoles.mockRejectedValueOnce(new Error('后端至少一个有效角色校验'))
    await findAriaButton(wrapper, '分配角色').trigger('click')
    await flushPromises()
    await bodyButton('清空').trigger('click')
    await bodyButton('保存').trigger('click')
    await flushPromises()
    expect(updateRoles).toHaveBeenLastCalledWith(7, { roleIds: [] })
    expect(document.body.textContent).toContain('后端至少一个有效角色校验')
  })

  it('confirms status and delete consequences then refreshes the applied page', async () => {
    const wrapper = mountPage(['user:account:status', 'user:account:delete'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '已禁用').trigger('click')
    await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenCalledWith(
      expect.stringContaining('重新登录'),
      expect.any(String),
      expect.any(Object),
    )
    expect(updateStatus).toHaveBeenCalledWith(7, YesNo.No)
    await findAriaButton(wrapper, '删除用户').trigger('click')
    await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenLastCalledWith(
      expect.stringContaining('新账号'),
      expect.any(String),
      expect.any(Object),
    )
    expect(deleteUser).toHaveBeenCalledWith(7)
  })

  it('keeps main as scroll owner and dialogs use body scrolling', async () => {
    const wrapper = mountPage(['user:account:authorize'], 9)
    await flushPromises()
    expect(wrapper.get('.user-management').attributes('style') ?? '').not.toContain('overflow')
    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    await findAriaButton(wrapper, '分配角色').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('.role-dialog-scroll')).not.toBeNull()
  })
})

function mountPage(permissions: string[], currentUserID = 7): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: permissions,
  })
  useAuthStore(pinia).setAuthenticated({
    userId: currentUserID,
    username: 'alice',
    email: 'alice@example.com',
    phone: '+86 138-0000-0000',
    avatar: '',
    passwordSetRequired: false,
  })
  return mount(UserManagement, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
}
function row() {
  return {
    id: 7,
    username: 'alice',
    email: 'alice@example.com',
    phone: '+86 138-0000-0000',
    isEnabled: YesNo.Yes,
    actions: { update: true, status: true, delete: true, authorize: true },
    actionLabels: { update: '编辑', status: '禁用', delete: '删除用户', authorize: '分配角色' },
    roles: roles(),
    createdAt: '2026-08-20T00:00:00Z',
    updatedAt: '2026-08-20T01:00:00Z',
  }
}
function roles() {
  return [
    {
      id: 3,
      code: 'ai_tester',
      name: 'AI Tester',
      isEnabled: YesNo.No,
      selectable: true,
      locked: false,
    },
    {
      id: 2,
      code: 'member',
      name: 'Member',
      isEnabled: YesNo.Yes,
      selectable: true,
      locked: false,
    },
  ].sort((a, b) => a.code.localeCompare(b.code) || a.id - b.id)
}
function findButton(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (button === undefined) throw new Error(`button ${text} missing`)
  return button
}
function findAriaButton(wrapper: VueWrapper, label: string) {
  const aria = wrapper.find(`button[aria-label="${label}"]`)
  if (aria.exists()) return aria
  return (
    wrapper.findAll('button').find((button) => button.text().includes(label)) ??
    wrapper.get(`button[aria-label="${label}"]`)
  )
}
function bodyButton(text: string) {
  const button = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find(
    (item) => item.textContent?.includes(text),
  )
  if (button === undefined) throw new Error(`body button ${text} missing`)
  return {
    attributes: (name: string) => button.getAttribute(name),
    trigger: async (event: string) => {
      button.click()
      await Promise.resolve(event)
    },
  }
}

function roleResponse(id: number, roleIds: number[]): userAPI.UserRolesResponse {
  return {
    user: {
      id,
      username: `user${id}`,
      email: `user${id}@example.com`,
      phone: null,
      isEnabled: YesNo.Yes,
    },
    roles: roles(),
    roleIds,
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}
