import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createMenu,
  deleteMenu,
  getMenus,
  updateMenu,
  updateMenuStatus,
} from '@src/api/menu'
import type { CreateMenuInput, UpdateMenuInput } from '@src/api/menu.contract'
import type { ManagedMenuNode } from '@src/api/menu.contract'
import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import { useAccessStore } from '@src/store/access'
import MenuManagement from '@src/views/system/menus/index.vue'

vi.mock('@src/api/menu', () => ({
  getMenus: vi.fn(),
  createMenu: vi.fn(),
  updateMenu: vi.fn(),
  updateMenuStatus: vi.fn(),
  deleteMenu: vi.fn(),
}))

const getMenusMock = vi.mocked(getMenus)
const createMenuMock = vi.mocked(createMenu)
const updateMenuMock = vi.mocked(updateMenu)
const updateMenuStatusMock = vi.mocked(updateMenuStatus)
const deleteMenuMock = vi.mocked(deleteMenu)

describe('MenuManagement', () => {
  let pinia: Pinia

  beforeEach(() => {
    document.body.innerHTML = ''
    pinia = createPinia()
    setActivePinia(pinia)
    localStorage.clear()
    setLocale('zh-CN')
    vi.clearAllMocks()
    getMenusMock.mockResolvedValue(menuTree())
  })

  it('loads once and renders the complete translated tree table', async () => {
    const wrapper = mountPage(pinia, ['system:menu:list'])
    await flushPromises()

    expect(getMenusMock).toHaveBeenCalledOnce()
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.menu-management-page').classes()).toContain('system-page')
    const table = wrapper.get('[data-testid="menu-table"]')
    expect(table.text()).toContain('系统管理')
    expect(table.text()).toContain('菜单管理')
    expect(table.text()).toContain('新增菜单')
    expect(table.text()).toContain('目录')
    expect(table.text()).toContain('页面')
    expect(table.text()).toContain('按钮权限')
    expect(table.text()).toContain('system:menu:list')
    expect(table.text()).toContain('/system/menus')
    expect(table.text()).toContain('system-menus')
    expect(table.text()).toContain('Setting')
    expect(table.text()).toContain('已启用')
    expect(table.text()).toContain('已禁用')
    expect(table.text()).toContain('是')
    const elementTable = wrapper.findComponent({ name: 'ElTable' })
    expect(elementTable.props('defaultExpandAll')).toBe(true)
    expect(elementTable.props('border')).toBe(true)
    expect(elementTable.props('headerCellStyle')).toEqual({ background: 'var(--el-fill-color-light)' })
    const centeredLabels = ['类型', '图标', '状态', '操作']
    const centeredColumns = wrapper.findAllComponents({ name: 'ElTableColumn' })
      .filter((column) => centeredLabels.includes(String(column.props('label'))))
    expect(centeredColumns).toHaveLength(centeredLabels.length)
    for (const column of centeredColumns) {
      expect(column.props('align')).toBe('center')
      expect(column.props('headerAlign')).toBe('center')
    }
    const actionsColumn = centeredColumns.find((column) => column.props('label') === '操作')
    expect(actionsColumn?.props('width')).toBe(280)
    expect(wrapper.find('.menu-row-actions').exists()).toBe(false)
    expect(wrapper.find('[data-testid="menu-drawer"]').exists()).toBe(false)
  })

  it('keeps disabled rows visible and accepts explicit empty leaf children', async () => {
    const wrapper = mountPage(pinia, ['system:menu:list'])
    await flushPromises()
    expect(wrapper.findAll('[data-menu-enabled="0"]')).toHaveLength(1)
    expect(menuTree()[0].children[0].children[0].children).toEqual([])
  })

  it('shows an explicit load failure instead of a fake empty success', async () => {
    getMenusMock.mockRejectedValue(new Error('postgres unavailable'))
    const wrapper = mountPage(pinia, ['system:menu:list'])
    await flushPromises()

    expect(wrapper.get('[data-testid="menu-load-error"]').text()).toContain('postgres unavailable')
    expect(wrapper.find('[data-testid="menu-empty"]').exists()).toBe(false)
  })

  it('uses create, update, and delete permissions independently', async () => {
    const createWrapper = mountPage(pinia, ['system:menu:create'])
    await flushPromises()
    expect(createWrapper.find('[data-testid="add-root-menu"]').exists()).toBe(true)
    expect(createWrapper.findAll('[data-testid^="add-child-"]').length).toBeGreaterThan(0)
    expect(createWrapper.find('[data-testid^="edit-"]').exists()).toBe(false)
    expect(createWrapper.find('[data-testid^="delete-"]').exists()).toBe(false)
    createWrapper.unmount()

    pinia = createPinia()
    const updateWrapper = mountPage(pinia, ['system:menu:update'])
    await flushPromises()
    expect(updateWrapper.find('[data-testid="add-root-menu"]').exists()).toBe(false)
    expect(updateWrapper.find('[data-testid^="edit-"]').exists()).toBe(true)
    expect(updateWrapper.find('[data-testid^="status-"]').exists()).toBe(true)
    expect(updateWrapper.find('[data-testid^="delete-"]').exists()).toBe(false)
    updateWrapper.unmount()

    pinia = createPinia()
    const deleteWrapper = mountPage(pinia, ['system:menu:delete'])
    await flushPromises()
    expect(deleteWrapper.find('[data-testid^="edit-"]').exists()).toBe(false)
    expect(deleteWrapper.find('[data-testid^="delete-"]').exists()).toBe(true)
  })

  it('keeps builtin mutation controls visibly disabled while edit remains available', async () => {
    const wrapper = mountPage(pinia, ['system:menu:update', 'system:menu:delete'])
    await flushPromises()

    expect(wrapper.get('[data-testid="edit-1"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="status-1"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="delete-1"]').attributes('disabled')).toBeDefined()
  })

  it('does not create a page or body-level scroll owner', async () => {
    const wrapper = mountPage(pinia, ['system:menu:list'])
    await flushPromises()
    expect(wrapper.get('.menu-management-page').classes()).not.toContain('admin-layout__scroll-owner')
    expect(wrapper.get('.menu-management-page').attributes('style') ?? '').not.toContain('overflow: auto')
  })

  it('creates a root with explicit nulls and reloads only the management tree', async () => {
    getMenusMock.mockResolvedValueOnce(menuTree()).mockResolvedValueOnce(menuTree())
    createMenuMock.mockResolvedValue({ id: 9 })
    const wrapper = mountPage(pinia, ['system:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-testid="menu-dialog"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="menu-drawer"]')).toBeNull()
    await bodyGet('[data-testid="menu-form-code"]').setValue('reports')
    await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
    await flushPromises()

    const expected: CreateMenuInput = {
      parentId: null,
      menuType: 'directory',
      code: 'reports',
      i18nKey: 'navigation.system',
      path: null,
      viewKey: null,
      icon: null,
      sortOrder: 100,
      isEnabled: YesNo.Yes,
    }
    expect(createMenuMock).toHaveBeenCalledWith(expected)
    expect(getMenusMock).toHaveBeenCalledTimes(2)
    expect(document.body.textContent ?? '').toContain('刷新页面后侧边栏和路由生效')
  })

  it('lets the menu dialog size itself to the form content', async () => {
    const wrapper = mountPage(pinia, ['system:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()

    expect(wrapper.findComponent({ name: 'AppDialog' }).props('height')).toBeUndefined()
  })

  it('shows only type-valid fields and clears incompatible values on type changes', async () => {
    const wrapper = mountPage(pinia, ['system:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()
    expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(false)
    expect(bodyFind('[data-testid="menu-form-view-key"]').exists()).toBe(false)

    await bodyGet('[data-testid="menu-form-type"]').trigger('click')
    const pageOption = [...document.body.querySelectorAll('.el-select-dropdown__item')].find((item) => item.textContent?.trim() === '页面')
    expect(pageOption).not.toBeNull()
    if (pageOption !== null) await new DOMWrapper(pageOption).trigger('click')
    await flushPromises()
    expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(true)
    expect(bodyFind('[data-testid="menu-form-view-key"]').exists()).toBe(true)

    await bodyGet('[data-testid="menu-form-path"]').setValue('/reports')
    await bodyGet('[data-testid="menu-form-type"]').trigger('click')
    const actionOption = Array.from(document.body.querySelectorAll('.el-select-dropdown__item')).find((item) => item.textContent?.includes('按钮权限'))
    expect(actionOption).not.toBeUndefined()
    if (actionOption !== undefined) await new DOMWrapper(actionOption).trigger('click')
    await flushPromises()
    expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(false)
    expect(bodyFind('[data-testid="menu-form-view-key"]').exists()).toBe(false)
  })

  it('keeps code readonly on edit and excludes it from the update payload', async () => {
    updateMenuMock.mockResolvedValue({ id: 2 })
    getMenusMock.mockResolvedValueOnce(menuTree()).mockResolvedValueOnce(menuTree())
    const wrapper = mountPage(pinia, ['system:menu:update'])
    await flushPromises()
    await wrapper.get('[data-testid="edit-2"]').trigger('click')
    await flushPromises()
    const codeInput = bodyGet('[data-testid="menu-form-code"]')
    expect(codeInput.attributes('readonly')).toBeDefined()
    await bodyGet('[data-testid="menu-form-sort-order"] input').setValue('12')
    await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
    await flushPromises()

    const expected: UpdateMenuInput = {
      parentId: 1,
      menuType: 'page',
      i18nKey: 'navigation.systemMenus',
      path: '/system/menus',
      viewKey: 'system-menus',
      icon: 'Menu',
      sortOrder: 12,
    }
    expect(updateMenuMock).toHaveBeenCalledWith(2, expected)
    expect(JSON.stringify(updateMenuMock.mock.calls[0]?.[1])).not.toContain('code')
  })

  it('filters parent options to valid nodes and excludes the edited subtree', async () => {
    getMenusMock.mockResolvedValue([rootWithEditableSubtree()])
    const wrapper = mountPage(pinia, ['system:menu:create', 'system:menu:update'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()
    await bodyGet('[data-testid="menu-form-type"]').trigger('click')
    const pageOption = [...document.body.querySelectorAll('.el-select-dropdown__item')].find((item) => item.textContent?.trim() === '页面')
    if (pageOption !== null) await new DOMWrapper(pageOption).trigger('click')
    await flushPromises()
    await bodyGet('[data-testid="menu-form-parent"]').trigger('click')
    const optionsText = document.body.querySelector('.el-select-dropdown')?.textContent ?? ''
    expect(optionsText).toContain('系统管理')
    expect(optionsText).toContain('reports')
    expect(optionsText).not.toContain('菜单管理')

    await bodyGet('[data-testid="menu-form-cancel"]').trigger('click')
    await wrapper.get('[data-testid="edit-10"]').trigger('click')
    await flushPromises()
    await bodyGet('[data-testid="menu-form-parent"]').trigger('click')
    const editOptionsText = document.body.querySelector('.el-select-dropdown')?.textContent ?? ''
    expect(editOptionsText).not.toContain('可编辑目录')
    expect(editOptionsText).not.toContain('可编辑页面')
  })

  it('uses exact status/delete APIs, reloads once, and preserves the table on failure', async () => {
    const mutableTree = menuTree()
    mutableTree[0].children[0].children[0].isBuiltin = false
    getMenusMock.mockResolvedValueOnce(mutableTree).mockResolvedValue(mutableTree)
    updateMenuStatusMock.mockResolvedValue({ id: 3, isEnabled: YesNo.Yes })
    deleteMenuMock.mockResolvedValue({ id: 3 })
    const wrapper = mountPage(pinia, ['system:menu:update', 'system:menu:delete'])
    await flushPromises()
    await wrapper.get('[data-testid="status-3"]').trigger('click')
    await flushPromises()
    expect(updateMenuStatusMock).toHaveBeenCalledWith(3, YesNo.Yes)
    expect(getMenusMock).toHaveBeenCalledTimes(2)

    getMenusMock.mockClear()
    await wrapper.get('[data-testid="delete-3"]').trigger('click')
    const confirmButton = document.body.querySelector('.el-message-box__btns .el-button--primary')
    expect(confirmButton).not.toBeNull()
    await confirmButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(deleteMenuMock).toHaveBeenCalledWith(3)
    expect(getMenusMock).toHaveBeenCalledOnce()

    updateMenuMock.mockRejectedValue(new Error('update failed'))
    getMenusMock.mockClear()
    await wrapper.get('[data-testid="edit-2"]').trigger('click')
    await flushPromises()
    await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
    await flushPromises()
    expect(getMenusMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="menu-table"]').text()).toContain('菜单管理')
    expect(wrapper.get('[data-testid="menu-mutation-error"]').text()).toContain('update failed')
  })
})

function mountPage(pinia: Pinia, permissions: string[]): VueWrapper {
  useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: permissions })
  return mount(MenuManagement, {
    attachTo: document.body,
    global: { plugins: [ElementPlus, appI18n, pinia] },
  })
}

function bodyFind(selector: string) {
  const element = document.body.querySelector(selector)
  return element === null ? { exists: () => false } : new DOMWrapper(element)
}

function bodyGet(selector: string) {
  const element = document.body.querySelector(selector)
  if (element === null) throw new Error(`Unable to find ${selector} in document.body`)
  return new DOMWrapper(element)
}

function menuTree(): ManagedMenuNode[] {
  const timestamp = '2026-08-19T02:00:00Z'
  return [{
    id: 1,
    parentId: null,
    menuType: 'directory',
    code: 'system',
    i18nKey: 'navigation.system',
    path: null,
    viewKey: null,
    icon: 'Setting',
    sortOrder: 100,
    isEnabled: YesNo.Yes,
    isBuiltin: true,
    createdAt: timestamp,
    updatedAt: timestamp,
    children: [{
      id: 2,
      parentId: 1,
      menuType: 'page',
      code: 'system:menu:list',
      i18nKey: 'navigation.systemMenus',
      path: '/system/menus',
      viewKey: 'system-menus',
      icon: 'Menu',
      sortOrder: 10,
      isEnabled: YesNo.Yes,
      isBuiltin: true,
      createdAt: timestamp,
      updatedAt: timestamp,
      children: [{
        id: 3,
        parentId: 2,
        menuType: 'action',
        code: 'system:menu:create',
        i18nKey: 'permission.menuCreate',
        path: null,
        viewKey: null,
        icon: null,
        sortOrder: 10,
        isEnabled: YesNo.No,
        isBuiltin: true,
        createdAt: timestamp,
        updatedAt: timestamp,
        children: [],
      }],
    }],
  }]
}

function rootWithEditableSubtree(): ManagedMenuNode {
  const root = menuTree()[0]
  const directory: ManagedMenuNode = {
    id: 10,
    parentId: null,
    menuType: 'directory',
    code: 'reports',
    i18nKey: 'navigation.system',
    path: null,
    viewKey: null,
    icon: 'Folder',
    sortOrder: 200,
    isEnabled: YesNo.Yes,
    isBuiltin: false,
    createdAt: root.createdAt,
    updatedAt: root.updatedAt,
    children: [{
      id: 11,
      parentId: 10,
      menuType: 'page',
      code: 'reports:list',
      i18nKey: 'navigation.systemMenus',
      path: '/reports',
      viewKey: 'system-menus',
      icon: 'Menu',
      sortOrder: 10,
      isEnabled: YesNo.Yes,
      isBuiltin: false,
      createdAt: root.createdAt,
      updatedAt: root.updatedAt,
      children: [],
    }],
  }
  return { ...root, children: [...root.children, directory] }
}
