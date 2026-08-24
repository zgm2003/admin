import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import { useAccessStore } from '@src/store/access'
import { useAuthStore } from '@src/store/auth'
import * as userAPI from '@src/api/user'
import UserManagement from '@src/views/system/users/index.vue'

vi.mock('@src/api/user', () => ({ getUsers:vi.fn(), getUserRoleOptions:vi.fn(), updateUser:vi.fn(), updateUserStatus:vi.fn(), deleteUser:vi.fn(), getUserRoles:vi.fn(), updateUserRoles:vi.fn() }))
const getUsers = vi.mocked(userAPI.getUsers)
const getRoleOptions = vi.mocked(userAPI.getUserRoleOptions)
const updateUser = vi.mocked(userAPI.updateUser)
const updateStatus = vi.mocked(userAPI.updateUserStatus)
const deleteUser = vi.mocked(userAPI.deleteUser)
const getUserRoles = vi.mocked(userAPI.getUserRoles)
const updateRoles = vi.mocked(userAPI.updateUserRoles)

describe('user management', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    getRoleOptions.mockResolvedValue({ roles: roles() })
    getUsers.mockResolvedValue({ list:[row()], total:1, page:1, pageSize:20 })
    updateUser.mockResolvedValue({ id:7, username:'new_name', updatedAt:'2026-08-20T02:00:00Z' })
    updateStatus.mockResolvedValue({ id:7, isEnabled:YesNo.No })
    deleteUser.mockResolvedValue({})
    getUserRoles.mockResolvedValue({ user:{id:7,username:'alice',email:'alice@example.com',isEnabled:YesNo.Yes}, roles:roles(), roleIds:[2,3] })
    updateRoles.mockResolvedValue({ id:7, roleCount:2 })
    vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () =>
      Object.assign('confirm' as const, { value: '', action: 'confirm' as const }),
    )
  })
  afterEach(() => { vi.restoreAllMocks(); document.body.innerHTML='' })

  it('loads, renders roles, and applies filters and paging', async () => {
    const wrapper = mountPage(['system:user:list'])
    await flushPromises()
    expect(getRoleOptions).toHaveBeenCalledTimes(1)
    expect(getUsers).toHaveBeenCalledWith({ page:1, pageSize:20 })
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.user-management').classes()).toContain('system-page')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('角色已禁用')
    await wrapper.get('.user-filters input').setValue(' alice ')
    const selects = wrapper.findAllComponents({ name:'ElSelectV2' })
    selects[0].vm.$emit('update:modelValue', YesNo.No)
    selects[1].vm.$emit('update:modelValue', 2)
    await findButton(wrapper, '查询').trigger('click'); await flushPromises()
    expect(getUsers).toHaveBeenLastCalledWith({ page:1, pageSize:20, keyword:'alice', isEnabled:YesNo.No, roleId:2 })
  })

  it('renders only granted commands and protects self and super targets', async () => {
    const wrapper = mountPage(['system:user:update','system:user:status','system:user:delete','system:user:roles'])
    await flushPromises()
    const texts = wrapper.findAll('button').map((button) => button.attributes('aria-label') ?? button.text())
    expect(texts.join(' ')).toContain('编辑')
    expect(texts.join(' ')).toContain('分配角色')
    const dangerous = wrapper.findAll('button').filter((button) => ['已禁用','删除用户','分配角色'].includes(button.text()))
    expect(dangerous.every((button) => button.attributes('disabled') !== undefined)).toBe(true)
  })

  it('edits the current username and synchronizes auth without loading access', async () => {
    const wrapper = mountPage(['system:user:update'])
    await flushPromises()
    const access = useAccessStore()
    const loadAccess = vi.spyOn(access, 'load')
    await findAriaButton(wrapper, '编辑').trigger('click'); await flushPromises()
    const input = document.body.querySelector<HTMLInputElement>('.user-edit-dialog input:not([disabled])')
    expect(input).not.toBeNull(); if (input === null) return
    input.value = ' new_name '; input.dispatchEvent(new Event('input'))
    await bodyButton('保存').trigger('click'); await flushPromises()
    expect(updateUser).toHaveBeenCalledWith(7, { username:'new_name' })
    expect(useAuthStore().user?.username).toBe('new_name')
    expect(loadAccess).not.toHaveBeenCalled()
  })

  it('loads and saves unique sorted user roles without refreshing access', async () => {
    const wrapper = mountPage(['system:user:roles'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '分配角色').trigger('click'); await flushPromises()
    expect(getUserRoles).toHaveBeenCalledWith(7)
    await bodyButton('全选').trigger('click')
    await bodyButton('保存').trigger('click'); await flushPromises()
    expect(updateRoles).toHaveBeenCalledWith(7, { roleIds:[2,3] })
    expect(vi.spyOn(useAccessStore(), 'load')).not.toHaveBeenCalled()
  })

  it('does not change the super administrator selection when an ordinary actor selects all roles', async () => {
    getUserRoles.mockResolvedValue({
      user: { id: 7, username: 'alice', email: 'alice@example.com', isEnabled: YesNo.Yes },
      roles: [
        { id: 3, code: 'ai_tester', name: 'AI Tester', isEnabled: YesNo.No },
        { id: 2, code: 'member', name: 'Member', isEnabled: YesNo.Yes },
        { id: 1, code: 'super_admin', name: 'Super Admin', isEnabled: YesNo.Yes },
      ],
      roleIds: [2],
    })
    const wrapper = mountPage(['system:user:roles'], 9)
    await flushPromises()
    await findAriaButton(wrapper, '分配角色').trigger('click'); await flushPromises()
    await bodyButton('全选').trigger('click')
    await bodyButton('保存').trigger('click'); await flushPromises()
    expect(updateRoles).toHaveBeenCalledWith(7, { roleIds: [2, 3] })
  })

  it('confirms status and delete consequences then refreshes the applied page', async () => {
    const wrapper = mountPage(['system:user:status','system:user:delete'], 9)
    await flushPromises()
		await findAriaButton(wrapper, '已禁用').trigger('click'); await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenCalledWith(expect.stringContaining('重新登录'), expect.any(String), expect.any(Object))
    expect(updateStatus).toHaveBeenCalledWith(7, YesNo.No)
		await findAriaButton(wrapper, '删除用户').trigger('click'); await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenLastCalledWith(expect.stringContaining('新账号'), expect.any(String), expect.any(Object))
    expect(deleteUser).toHaveBeenCalledWith(7)
  })

  it('keeps main as scroll owner and dialogs use body scrolling', async () => {
    const wrapper = mountPage(['system:user:roles'], 9); await flushPromises()
    expect(wrapper.get('.user-management').attributes('style') ?? '').not.toContain('overflow')
    await findAriaButton(wrapper, '分配角色').trigger('click'); await flushPromises()
    expect(document.body.querySelector('.role-dialog-scroll')).not.toBeNull()
  })
})

function mountPage(permissions: string[], currentUserID = 7): VueWrapper {
  const pinia = createPinia(); setActivePinia(pinia)
  useAccessStore(pinia).applySnapshot({ roleCodes:[], menuTree:[], permissionCodes:permissions })
  useAuthStore(pinia).setAuthenticated({ userId:currentUserID, username:'alice', email:'alice@example.com' })
  return mount(UserManagement, { attachTo:document.body, global:{ plugins:[pinia, appI18n, ElementPlus] } })
}
function row() { return { id:7, username:'alice', email:'alice@example.com', isEnabled:YesNo.Yes, roles:roles(), createdAt:'2026-08-20T00:00:00Z', updatedAt:'2026-08-20T01:00:00Z' } }
function roles() { return [{id:3,code:'ai_tester',name:'AI Tester',isEnabled:YesNo.No},{id:2,code:'member',name:'Member',isEnabled:YesNo.Yes}].sort((a,b)=>a.code.localeCompare(b.code)||a.id-b.id) }
function findButton(wrapper: VueWrapper, text: string) { const button=wrapper.findAll('button').find((item)=>item.text().includes(text)); if(button===undefined) throw new Error(`button ${text} missing`); return button }
function findAriaButton(wrapper: VueWrapper, label: string) {
  const aria = wrapper.find(`button[aria-label="${label}"]`)
  if (aria.exists()) return aria
  return wrapper.findAll('button').find((button) => button.text().includes(label)) ?? wrapper.get(`button[aria-label="${label}"]`)
}
function bodyButton(text: string) { const button=Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find((item)=>item.textContent?.includes(text)); if(button===undefined) throw new Error(`body button ${text} missing`); return { trigger: async (event:string) => { button.click(); await Promise.resolve(event) } } }
