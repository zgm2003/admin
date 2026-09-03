import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import ElementPlus, { ElCheckbox, ElPagination, ElSelectV2, ElTooltip } from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
import type { RoleListItem } from '@/api/permission/role'
import { YesNo } from '@/enums/yes-no'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import RolePermissionMatrix from '@/views/permission/roles/components/RolePermissionMatrix/index.vue'
import RoleManagement from '@/views/permission/roles/index.vue'

vi.mock('@/api/permission/role', () => ({
  getRoles: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  updateRoleStatus: vi.fn(),
  setDefaultRole: vi.fn(),
  deleteRole: vi.fn(),
  getRolePermissions: vi.fn(),
  updateRolePermissions: vi.fn(),
}))

const getRolesMock = vi.mocked(getRoles)
const createRoleMock = vi.mocked(createRole)
const updateRoleMock = vi.mocked(updateRole)
const updateRoleStatusMock = vi.mocked(updateRoleStatus)
const setDefaultRoleMock = vi.mocked(setDefaultRole)
const deleteRoleMock = vi.mocked(deleteRole)
const getRolePermissionsMock = vi.mocked(getRolePermissions)
const updateRolePermissionsMock = vi.mocked(updateRolePermissions)

describe('RoleManagement', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    localStorage.clear()
    setLocale('zh-CN')
    vi.clearAllMocks()
    getRolesMock.mockResolvedValue({
      list: [
        {
          id: 3,
          code: 'tester',
          name: '测试员',
          isDefault: YesNo.No,
          isEnabled: YesNo.Yes,
          userCount: 2,
          permissionCount: 1,
          createdAt: '2026-08-19T00:00:00Z',
          updatedAt: '2026-08-19T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    createRoleMock.mockResolvedValue({ id: 7 })
    updateRoleMock.mockResolvedValue({})
    updateRoleStatusMock.mockResolvedValue({ id: 3, isEnabled: YesNo.No })
    setDefaultRoleMock.mockResolvedValue({ id: 3, isDefault: YesNo.Yes })
    deleteRoleMock.mockResolvedValue({})
    getRolePermissionsMock.mockResolvedValue(permissionResponse())
    updateRolePermissionsMock.mockResolvedValue({ id: 3, permissionCount: 1 })
  })

  it('loads explicit pagination once and renders the role row', async () => {
    const wrapper = mountPage(['permission:role:list'])
    await flushPromises()

    expect(getRolesMock).toHaveBeenCalledOnce()
    expect(getRolesMock).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.role-page').classes()).toContain('management-page')
    expect(wrapper.find('[aria-label="角色管理"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('测试员')
    expect(wrapper.text()).toContain('tester')
  })

  it('renders commands only for their exact permissions', async () => {
    const wrapper = mountPage(['permission:role:create'])
    await flushPromises()

    expect(wrapper.text()).toContain('新增角色')
    expect(wrapper.text()).not.toContain('配置角色权限')
    expect(wrapper.find('.role-page').attributes('style') ?? '').not.toContain('overflow')
  })

  it.each([
    ['permission:role:create', '新增角色'],
    ['permission:role:update', '编辑'],
    ['permission:role:status', '禁用'],
    ['permission:role:default', '设为默认'],
    ['permission:role:delete', '删除'],
    ['permission:role:authorize', '授权'],
  ])('shows only the command granted by %s', async (permission, expectedCommand) => {
    getRolesMock.mockResolvedValue({
      list: [roleItem({ userCount: 0 })],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage([permission])
    await flushPromises()

    const renderedCommands = [
      ...wrapper.findAll('button').map((button) => button.text()),
      ...wrapper.findAllComponents(ElTooltip).map((tooltip) => String(tooltip.props('content'))),
    ]
    const roleCommands = ['新增角色', '编辑', '禁用', '设为默认', '删除', '授权']
    expect(renderedCommands).toContain(expectedCommand)
    for (const command of roleCommands.filter((candidate) => candidate !== expectedCommand)) {
      expect(renderedCommands).not.toContain(command)
    }
  })

  it('keeps prior rows visible when a refresh fails', async () => {
    getRolesMock
      .mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })
      .mockRejectedValueOnce(new Error('postgres unavailable'))
    const wrapper = mountPage(['permission:role:list'])
    await flushPromises()
    const buttons = wrapper.findAll('button')
    const refresh = buttons.find((button) => button.text().includes('刷新'))
    expect(refresh).toBeDefined()
    await refresh?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('postgres unavailable')
  })

  it('resets filters to page one and preserves them while paging', async () => {
    const wrapper = mountPage(['permission:role:list'])
    await flushPromises()

    await wrapper.get('.role-filters input').setValue(' tester ')
    const statusSelect = wrapper.getComponent(ElSelectV2)
    statusSelect.vm.$emit('update:modelValue', YesNo.No)
    await findButton(wrapper, '查询').trigger('click')
    await flushPromises()
    expect(getRolesMock).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: 'tester',
      isEnabled: YesNo.No,
    })

    wrapper.getComponent(ElPagination).vm.$emit('current-change', 3)
    await flushPromises()
    expect(getRolesMock).toHaveBeenLastCalledWith({
      page: 3,
      pageSize: 20,
      keyword: 'tester',
      isEnabled: YesNo.No,
    })

    await findButton(wrapper, '重置').trigger('click')
    await flushPromises()
    expect(getRolesMock).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })
  })

  it('creates from the profile dialog and returns the list to page one', async () => {
    const wrapper = mountPage(['permission:role:create'])
    await flushPromises()
    wrapper.getComponent(ElPagination).vm.$emit('current-change', 3)
    await flushPromises()

    await findButton(wrapper, '新增角色').trigger('click')
    await flushPromises()
    const inputs = bodyDialogInputs()
    expect(inputs).toHaveLength(2)
    await inputs[0].setValue('ai_tester')
    await inputs[1].setValue('AI 测试员')
    await bodyButton('保存').trigger('click')
    await flushPromises()

    expect(createRoleMock).toHaveBeenCalledWith({ code: 'ai_tester', name: 'AI 测试员' })
    expect(getRolesMock).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })
  })

  it('keeps code readonly and preserves filters while editing the role name', async () => {
    const wrapper = mountPage(['permission:role:update'])
    await flushPromises()
    await wrapper.get('.role-filters input').setValue('tester')
    await findButton(wrapper, '查询').trigger('click')
    await flushPromises()
    wrapper.getComponent(ElPagination).vm.$emit('current-change', 2)
    await flushPromises()

    await tooltipButton(wrapper, '编辑').trigger('click')
    await flushPromises()
    const inputs = bodyDialogInputs()
    expect(inputs[0].attributes('disabled')).toBeDefined()
    await inputs[1].setValue('测试工程师')
    await bodyButton('保存').trigger('click')
    await flushPromises()

    expect(updateRoleMock).toHaveBeenCalledWith(3, { name: '测试工程师' })
    expect(getRolesMock).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      keyword: 'tester',
    })
  })

  it('shows status, default, and delete impact before mutating', async () => {
    getRolesMock.mockResolvedValue({
      list: [roleItem({ userCount: 2, permissionCount: 0 })],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage([
      'permission:role:status',
      'permission:role:default',
      'permission:role:delete',
    ])
    await flushPromises()

    const statusRequest = tooltipButton(wrapper, '禁用').trigger('click')
    await flushPromises()
    expect(messageBoxText()).toContain('2 个关联用户')
    expect(messageBoxText()).toContain('用户关系和角色授权会保留')
    await confirmMessageBox()
    await statusRequest
    await flushPromises()
    expect(updateRoleStatusMock).toHaveBeenCalledWith(3, YesNo.No)

    const defaultRequest = tooltipButton(wrapper, '设为默认').trigger('click')
    await flushPromises()
    expect(messageBoxText()).toContain('只影响后续新用户')
    expect(messageBoxText()).toContain('只能访问固定工作台')
    await confirmMessageBox()
    await defaultRequest
    await flushPromises()
    expect(setDefaultRoleMock).toHaveBeenCalledWith(3)

    expect(
      tooltipButton(wrapper, '该角色仍绑定 2 个用户，不能删除。').attributes('disabled'),
    ).toBeDefined()
  })

  it('soft deletes after confirmation and moves back only when the current page becomes invalid', async () => {
    getRolesMock.mockResolvedValue({
      list: [roleItem({ userCount: 0 })],
      total: 21,
      page: 2,
      pageSize: 20,
    })
    const wrapper = mountPage(['permission:role:delete'])
    await flushPromises()
    wrapper.getComponent(ElPagination).vm.$emit('current-change', 2)
    await flushPromises()

    const deletion = tooltipButton(wrapper, '删除').trigger('click')
    await flushPromises()
    expect(messageBoxText()).toContain('软删除')
    await confirmMessageBox()
    await deletion
    await flushPromises()

    expect(deleteRoleMock).toHaveBeenCalledWith(3)
    expect(getRolesMock).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })
  })

  it('expands minimal action grants into a fully selected effective matrix', async () => {
    getRolePermissionsMock.mockResolvedValue(permissionResponse({ menuIds: [3, 8] }))
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()

    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()
    expect(getRolePermissionsMock).toHaveBeenCalledWith(3)
    expect(document.body.textContent).toContain('测试员 (tester)')
    expect(document.body.textContent).toContain('已禁用')
    expect(
      document.body.querySelector('[data-testid="role-permission-platform-tabs"]')?.textContent,
    ).toContain('Admin')
    expect(
      document.body.querySelector('[data-testid="role-permission-platform-tabs"]')?.textContent,
    ).toContain('Canvas')

    const matrix = wrapper.getComponent(RolePermissionMatrix)
    expect(matrix.props('modelValue')).toEqual([2, 3, 7, 8])
    const groupCheckbox = matrix
      .findAllComponents(ElCheckbox)
      .find((checkbox) => checkbox.text().includes('系统管理'))
    expect(groupCheckbox?.props('modelValue')).toBe(true)
    expect(groupCheckbox?.props('indeterminate')).toBe(false)
  })

  it('switches platform tabs and renders the Canvas root page matrix', async () => {
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    const canvasTab = Array.from(document.body.querySelectorAll<HTMLElement>('[role="tab"]')).find(
      (tab) => tab.textContent?.includes('Canvas'),
    )
    expect(canvasTab).toBeDefined()
    if (canvasTab === undefined) throw new Error('Canvas permission tab is missing')
    await new DOMWrapper(canvasTab).trigger('click')
    await flushPromises()

    const matrix = wrapper.getComponent(RolePermissionMatrix)
    expect(matrix.text()).toContain('Test')
    expect(matrix.text()).toContain('canvas:test:list')
    expect(matrix.text()).toContain('Test Button')
    expect(matrix.text()).not.toContain('系统管理')
  })

  it('shows the effective permission diff before submitting minimal direct grants', async () => {
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    const accessStore = usePermissionStore()
    const loadAccess = vi.spyOn(accessStore, 'load')
    const resetAccess = vi.spyOn(accessStore, 'reset')
    wrapper.getComponent(RolePermissionMatrix).vm.$emit('update:modelValue', [2, 3])
    await flushPromises()
    await bodyButton('保存授权').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('确认权限变更')
    expect(document.body.textContent).toContain('新增权限')
    expect(document.body.textContent).toContain('新增角色 · permission:role:create')
    expect(document.body.textContent).toContain('暂无权限变更')

    await bodyButton('确认').trigger('click')
    await flushPromises()
    expect(updateRolePermissionsMock).toHaveBeenCalledWith(3, { menuIds: [3] })
    expect(accessStore.permissionCodes).toEqual(['permission:role:authorize'])
    expect(loadAccess).not.toHaveBeenCalled()
    expect(resetAccess).not.toHaveBeenCalled()
  })

  it('selects and clears the complete matrix from the authorization toolbar', async () => {
    getRolePermissionsMock.mockResolvedValue(permissionResponse({ menuIds: [8] }))
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    const matrix = wrapper.getComponent(RolePermissionMatrix)
    await bodyButton('全选').trigger('click')
    await flushPromises()
    expect(matrix.props('modelValue')).toEqual([2, 3, 7, 8])

    await bodyButton('清空').trigger('click')
    await flushPromises()
    expect(matrix.props('modelValue')).toEqual([7, 8])
  })

  it('keeps the matrix selection when permission diff confirmation is cancelled', async () => {
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    const matrix = wrapper.getComponent(RolePermissionMatrix)
    matrix.vm.$emit('update:modelValue', [2, 3])
    await flushPromises()
    await bodyButton('保存授权').trigger('click')
    await flushPromises()
    await dialogButton('.role-permission-diff-dialog', '取消').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).not.toHaveBeenCalled()
    expect(matrix.props('modelValue')).toEqual([2, 3])
  })

  it('closes an unchanged authorization without sending a write', async () => {
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    await bodyButton('保存授权').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).not.toHaveBeenCalled()
    expect(document.body.textContent).not.toContain('确认权限变更')
    const dialog = document.body.querySelector('.role-permission-dialog')
    const overlay = dialog?.closest<HTMLElement>('.el-overlay')
    expect(overlay?.style.display).toBe('none')
  })

  it('confirms and submits an empty direct grant set as a valid authorization', async () => {
    updateRolePermissionsMock.mockResolvedValue({ id: 3, permissionCount: 0 })
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    wrapper.getComponent(RolePermissionMatrix).vm.$emit('update:modelValue', [])
    await flushPromises()
    await bodyButton('保存授权').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('移除权限')
    await bodyButton('确认').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).toHaveBeenCalledWith(3, { menuIds: [] })
  })

  it('keeps both dialogs and the effective selection after a save failure', async () => {
    updateRolePermissionsMock.mockRejectedValue(new Error('save failed'))
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    const matrix = wrapper.getComponent(RolePermissionMatrix)
    matrix.vm.$emit('update:modelValue', [2, 3])
    await flushPromises()
    await bodyButton('保存授权').trigger('click')
    await flushPromises()
    await bodyButton('确认').trigger('click')
    await flushPromises()

    expect(updateRolePermissionsMock).toHaveBeenCalledWith(3, { menuIds: [3] })
    expect(document.body.textContent).toContain('save failed')
    expect(document.body.textContent).toContain('确认权限变更')
    expect(document.body.textContent).toContain('测试员 (tester)')
    expect(matrix.props('modelValue')).toEqual([2, 3])
  })

  it('shows an authorization load error and retries the same role', async () => {
    getRolePermissionsMock
      .mockRejectedValueOnce(new Error('permission load failed'))
      .mockResolvedValueOnce(permissionResponse())
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('permission load failed')
    await bodyButton('重试').trigger('click')
    await flushPromises()

    expect(getRolePermissionsMock).toHaveBeenCalledTimes(2)
    expect(getRolePermissionsMock).toHaveBeenLastCalledWith(3)
    expect(document.body.textContent).toContain('测试员 (tester)')
  })

  it('keeps page and dialog scrolling in their approved owners', async () => {
    const wrapper = mountPage(['permission:role:authorize'])
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    expect(wrapper.get('.role-page').attributes('style') ?? '').not.toContain('overflow')

    await tooltipButton(wrapper, '授权').trigger('click')
    await flushPromises()
    const scroll = document.body.querySelector('.permission-scroll')
    const footer = document.body.querySelector('.el-dialog__footer')
    expect(scroll).not.toBeNull()
    expect(scroll?.classList.contains('permission-scroll')).toBe(true)
    expect(scroll?.contains(footer)).toBe(false)
  })

  it('explains every protected role action in its tooltip', async () => {
    getRolesMock.mockResolvedValue({
      list: [
        roleItem({ id: 1, code: 'super_admin', name: '超级管理员' }),
        roleItem({ id: 2, code: 'registered_user', name: '普通用户' }),
        roleItem({ id: 3, code: 'default_role', name: '默认角色', isDefault: YesNo.Yes }),
        roleItem({ id: 4, code: 'disabled_role', name: '禁用角色', isEnabled: YesNo.No }),
        roleItem({ id: 5, code: 'attached_role', name: '已绑定角色', userCount: 2 }),
      ],
      total: 5,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage([
      'permission:role:update',
      'permission:role:status',
      'permission:role:default',
      'permission:role:delete',
      'permission:role:authorize',
    ])
    await flushPromises()

    const tooltipContents = wrapper
      .findAllComponents(ElTooltip)
      .map((tooltip) => String(tooltip.props('content')))

    expect(tooltipContents).toContain('系统角色名称固定，不能编辑。')
    expect(tooltipContents).toContain('超级管理员必须保持启用。')
    expect(tooltipContents).toContain('超级管理员不能设为默认角色。')
    expect(tooltipContents).not.toContain('超级管理员通过固定规则拥有全部权限，无需配置。')
    expect(tooltipContents.filter((content) => content === '授权')).toHaveLength(4)
    expect(tooltipContents).toContain('系统角色不能删除。')
    expect(tooltipContents).toContain('默认角色不能禁用。')
    expect(tooltipContents).toContain('当前角色已经是默认角色。')
    expect(tooltipContents).toContain('请先启用角色，再将其设为默认角色。')
    expect(tooltipContents).toContain('该角色仍绑定 2 个用户，不能删除。')
  })
})

function mountPage(permissionCodes: string[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  return mount(RoleManagement, {
    attachTo: document.body,
    global: {
      plugins: [pinia, appI18n, ElementPlus],
    },
  })
}

function roleItem(overrides: Partial<RoleListItem>): RoleListItem {
  return { ...baseRoleItem(), ...overrides }
}

function baseRoleItem(): RoleListItem {
  return {
    id: 3,
    code: 'tester',
    name: '测试员',
    isDefault: YesNo.No,
    isEnabled: YesNo.Yes,
    userCount: 0,
    permissionCount: 1,
    createdAt: '2026-08-19T00:00:00Z',
    updatedAt: '2026-08-19T00:00:00Z',
  }
}

function permissionResponse(overrides: { menuIds?: number[] } = {}) {
  return {
    role: {
      id: 3,
      code: 'tester',
      name: '测试员',
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
            id: 1,
            parentId: null,
            menuType: 'directory' as const,
            code: 'system',
            name: '系统管理',
            isEnabled: YesNo.Yes,
            children: [
              {
                id: 2,
                parentId: 1,
                menuType: 'page' as const,
                code: 'permission:role:list',
                name: '角色管理',
                isEnabled: YesNo.Yes,
                children: [
                  {
                    id: 3,
                    parentId: 2,
                    menuType: 'action' as const,
                    code: 'permission:role:create',
                    name: '新增角色',
                    isEnabled: YesNo.No,
                    children: [],
                  },
                ],
              },
            ],
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
            id: 7,
            parentId: null,
            menuType: 'page' as const,
            code: 'canvas:test:list',
            name: 'Test',
            isEnabled: YesNo.Yes,
            children: [
              {
                id: 8,
                parentId: 7,
                menuType: 'action' as const,
                code: 'canvas:test:button',
                name: 'Test Button',
                isEnabled: YesNo.Yes,
                children: [],
              },
            ],
          },
        ],
      },
    ],
    menuIds: overrides.menuIds ?? [2],
  }
}

function findButton(wrapper: ReturnType<typeof mountPage>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  if (button === undefined) {
    throw new Error(`Unable to find button containing ${text}`)
  }
  return button
}

function tooltipButton(wrapper: ReturnType<typeof mountPage>, content: string) {
  const tooltip = wrapper
    .findAllComponents(ElTooltip)
    .find((candidate) => candidate.props('content') === content)
  if (tooltip === undefined) {
    throw new Error(`Unable to find tooltip containing ${content}`)
  }
  return tooltip.get('button')
}

function bodyDialogInputs(): DOMWrapper<HTMLInputElement>[] {
  return Array.from(document.body.querySelectorAll<HTMLInputElement>('.el-dialog input')).map(
    (element) => new DOMWrapper(element),
  )
}

function bodyButton(text: string): DOMWrapper<HTMLButtonElement> {
  const button = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find(
    (candidate) => candidate.textContent?.includes(text),
  )
  if (button === undefined) {
    throw new Error(`Unable to find body button containing ${text}`)
  }
  return new DOMWrapper(button)
}

function dialogButton(selector: string, text: string): DOMWrapper<HTMLButtonElement> {
  const dialog = document.body.querySelector(selector)
  const button = Array.from(dialog?.querySelectorAll<HTMLButtonElement>('button') ?? []).find(
    (candidate) => candidate.textContent?.includes(text),
  )
  if (button === undefined) {
    throw new Error(`Unable to find ${selector} button containing ${text}`)
  }
  return new DOMWrapper(button)
}

function messageBoxText(): string {
  const boxes = document.body.querySelectorAll('.el-message-box')
  return boxes.item(boxes.length - 1).textContent ?? ''
}

async function confirmMessageBox(): Promise<void> {
  const buttons = document.body.querySelectorAll<HTMLButtonElement>(
    '.el-message-box__btns .el-button--primary',
  )
  const button = buttons.item(buttons.length - 1)
  if (button === null) {
    throw new Error('Unable to find message box confirm button')
  }
  await new DOMWrapper(button).trigger('click')
}
