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
import { DIcon } from '@src/components/DIcon'
import { IconSelect } from '@src/components/IconSelect'
import { useAccessStore } from '@src/store/access'
import MenuManagement from '@src/views/access/menus/index.vue'

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

  it('loads once and renders the complete database-named tree table', async () => {
    const wrapper = mountPage(pinia, ['rbac:menu:list'])
    await flushPromises()

    expect(getMenusMock).toHaveBeenCalledOnce()
    expect(wrapper.find('h1').exists()).toBe(false)
		expect(wrapper.get('.menu-management-page').classes()).toContain('management-page')
    const table = wrapper.get('[data-testid="menu-table"]')
    expect(table.text()).toContain('系统管理')
		expect(table.text()).toContain('用户管理')
		expect(table.text()).toContain('修改用户')
    expect(table.text()).toContain('目录')
    expect(table.text()).toContain('页面')
    expect(table.text()).toContain('按钮权限')
		expect(table.text()).toContain('account:user:list')
		expect(table.text()).toContain('/account/users')
		expect(table.text()).toContain('account/users')
    expect(table.text()).toContain('lucide:settings-2')
    expect(table.text()).toContain('已启用')
    expect(table.text()).toContain('已禁用')
		expect(table.text()).toContain('显示')
		expect(table.text()).toContain('隐藏')
		expect(table.text()).not.toContain('内置')
    const elementTable = wrapper.findComponent({ name: 'ElTable' })
    expect(elementTable.props('defaultExpandAll')).toBe(false)
    expect(elementTable.props('expandRowKeys')).toEqual([1])
    expect(elementTable.props('border')).toBe(true)
    expect(elementTable.props('headerCellStyle')).toEqual({ background: 'var(--el-fill-color-light)' })
		const centeredLabels = ['类型', '图标', '显示状态', '状态', '操作']
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
    const wrapper = mountPage(pinia, ['rbac:menu:list'])
    await flushPromises()
    expect(wrapper.findAll('[data-menu-enabled="0"]')).toHaveLength(1)
    expect(menuTree()[0].children[0].children[0].children).toEqual([])
  })

  it('shows an explicit load failure instead of a fake empty success', async () => {
    getMenusMock.mockRejectedValue(new Error('postgres unavailable'))
    const wrapper = mountPage(pinia, ['rbac:menu:list'])
    await flushPromises()

    expect(wrapper.get('[data-testid="menu-load-error"]').text()).toContain('postgres unavailable')
    expect(wrapper.find('[data-testid="menu-empty"]').exists()).toBe(false)
  })

  it('uses create, update, and delete permissions independently', async () => {
    const createWrapper = mountPage(pinia, ['rbac:menu:create'])
    await flushPromises()
    expect(createWrapper.find('[data-testid="add-root-menu"]').exists()).toBe(true)
    expect(createWrapper.findAll('[data-testid^="add-child-"]').length).toBeGreaterThan(0)
    expect(createWrapper.find('[data-testid^="edit-"]').exists()).toBe(false)
    expect(createWrapper.find('[data-testid^="delete-"]').exists()).toBe(false)
    createWrapper.unmount()

    pinia = createPinia()
    const updateWrapper = mountPage(pinia, ['rbac:menu:update'])
    await flushPromises()
    expect(updateWrapper.find('[data-testid="add-root-menu"]').exists()).toBe(false)
    expect(updateWrapper.find('[data-testid^="edit-"]').exists()).toBe(true)
    expect(updateWrapper.find('[data-testid^="status-"]').exists()).toBe(true)
    expect(updateWrapper.find('[data-testid^="delete-"]').exists()).toBe(false)
    updateWrapper.unmount()

    pinia = createPinia()
    const deleteWrapper = mountPage(pinia, ['rbac:menu:delete'])
    await flushPromises()
    expect(deleteWrapper.find('[data-testid^="edit-"]').exists()).toBe(false)
    expect(deleteWrapper.find('[data-testid^="delete-"]').exists()).toBe(true)
  })

	it('allows every ordinary menu record to be edited, disabled, and deleted', async () => {
    const wrapper = mountPage(pinia, ['rbac:menu:update', 'rbac:menu:delete'])
    await flushPromises()

		expect(wrapper.get('[data-testid="edit-1"]').attributes('disabled')).toBeUndefined()
		expect(wrapper.get('[data-testid="status-1"]').attributes('disabled')).toBeUndefined()
		expect(wrapper.get('[data-testid="delete-1"]').attributes('disabled')).toBeUndefined()
  })

  it('does not create a page or body-level scroll owner', async () => {
    const wrapper = mountPage(pinia, ['rbac:menu:list'])
    await flushPromises()
    expect(wrapper.get('.menu-management-page').classes()).not.toContain('admin-layout__scroll-owner')
    expect(wrapper.get('.menu-management-page').attributes('style') ?? '').not.toContain('overflow: auto')
  })

  it('creates a root with explicit nulls and reloads only the management tree', async () => {
    getMenusMock.mockResolvedValueOnce(menuTree()).mockResolvedValueOnce(menuTree())
    createMenuMock.mockResolvedValue({ id: 9 })
    const wrapper = mountPage(pinia, ['rbac:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-testid="menu-dialog"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="menu-drawer"]')).toBeNull()
    await bodyGet('[data-testid="menu-form-code"]').setValue('reports')
		await bodyGet('[data-testid="menu-form-name"]').setValue('报表')
    await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
    await flushPromises()

    const expected: CreateMenuInput = {
      parentId: null,
      menuType: 'directory',
			name: '报表',
      code: 'reports',
      i18nKey: 'navigation.system',
      path: null,
		componentPath: null,
      icon: null,
      sortOrder: 100,
      isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
    }
		expect(createMenuMock).toHaveBeenCalledWith(expected)
    expect(getMenusMock).toHaveBeenCalledTimes(2)
    expect(document.body.textContent ?? '').toContain('刷新页面后侧边栏和路由生效')
	})

	it('locks protected structure, status, and deletion while keeping presentation fields editable', async () => {
		getMenusMock.mockResolvedValue(protectedMenuTree())
		const wrapper = mountPage(pinia, ['rbac:menu:update', 'rbac:menu:delete'])
		await flushPromises()

		expect(wrapper.get('[data-testid="status-2"]').attributes('disabled')).toBeDefined()
		expect(wrapper.get('[data-testid="delete-2"]').attributes('disabled')).toBeDefined()
		await wrapper.get('[data-testid="edit-2"]').trigger('click')
		await flushPromises()
		expect(document.body.querySelector('[data-testid="menu-form-protected-hint"]')).not.toBeNull()
		expect(bodyGet('[data-testid="menu-form-code"]').attributes('disabled')).toBeDefined()
		expect(bodyGet('[data-testid="menu-form-path"]').attributes('disabled')).toBeDefined()
		expect(bodyGet('[data-testid="menu-form-component-path"]').attributes('disabled')).toBeDefined()
		expect(bodyGet('[data-testid="menu-form-hidden"]').classes()).toContain('is-disabled')
		expect(bodyGet('[data-testid="menu-form-sort-order"]').attributes('disabled')).toBeUndefined()
	})

	it('stores an IconSelect value as a string and previews it with DIcon', async () => {
		createMenuMock.mockResolvedValue({ id: 9 })
		const wrapper = mountPage(pinia, ['rbac:menu:create'])
		await flushPromises()
		await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
		await flushPromises()
		wrapper.findComponent(IconSelect).vm.$emit('select-icon', 'lucide:shield-check')
		await flushPromises()

		expect(wrapper.findAllComponents(DIcon).some((icon) => icon.props('icon') === 'lucide:shield-check')).toBe(true)
		await bodyGet('[data-testid="menu-form-name"]').setValue('报表')
		await bodyGet('[data-testid="menu-form-code"]').setValue('reports')
		await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
		await flushPromises()
		expect(createMenuMock.mock.calls[0]?.[0].icon).toBe('lucide:shield-check')
	})

	it('forces action menus hidden without displaying a visibility control', async () => {
		createMenuMock.mockResolvedValue({ id: 9 })
		const wrapper = mountPage(pinia, ['rbac:menu:create'])
		await flushPromises()
		await wrapper.get('[data-testid="add-child-2"]').trigger('click')
		await flushPromises()

		expect(bodyFind('[data-testid="menu-form-hidden"]').exists()).toBe(false)
		expect(bodyFind('[data-testid="menu-form-i18n-key"]').exists()).toBe(false)
		await bodyGet('[data-testid="menu-form-name"]').setValue('新增用户')
		await bodyGet('[data-testid="menu-form-code"]').setValue('account:user:create')
		await bodyGet('[data-testid="menu-form-submit"]').trigger('click')
		await flushPromises()
		expect(createMenuMock).toHaveBeenCalledWith({
			parentId: 2,
			menuType: 'action',
			name: '新增用户',
			code: 'account:user:create',
			i18nKey: null,
			path: null,
			componentPath: null,
			icon: null,
			sortOrder: 100,
			isEnabled: YesNo.Yes,
			isHidden: YesNo.Yes,
		})
	})

  it('lets the menu dialog size itself to the form content', async () => {
    const wrapper = mountPage(pinia, ['rbac:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()

    expect(wrapper.findComponent({ name: 'AppDialog' }).props('height')).toBeUndefined()
  })

	it('uses text protocol fields with exact hints and clears incompatible values without deriving paths', async () => {
    const wrapper = mountPage(pinia, ['rbac:menu:create'])
    await flushPromises()
    await wrapper.get('[data-testid="add-root-menu"]').trigger('click')
    await flushPromises()
		expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(false)
		expect(bodyFind('[data-testid="menu-form-component-path"]').exists()).toBe(false)
		const i18nKeyInput = bodyGet('[data-testid="menu-form-i18n-key"]')
		expect(i18nKeyInput.element.tagName).toBe('INPUT')
		expect(i18nKeyInput.element.closest('.el-select-v2')).toBeNull()
		expect(document.body.textContent).toContain('i18nKey：至少两段点号路径，例如 navigation.accountUsers')
		expect(document.body.textContent).toContain('权限码：小写冒号分段，例如 account:user:list')

    await bodyGet('[data-testid="menu-form-type"]').trigger('click')
    const pageOption = [...document.body.querySelectorAll('.el-select-dropdown__item')].find((item) => item.textContent?.trim() === '页面')
    expect(pageOption).not.toBeNull()
    if (pageOption !== null) await new DOMWrapper(pageOption).trigger('click')
    await flushPromises()
		expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(true)
		expect(bodyFind('[data-testid="menu-form-component-path"]').exists()).toBe(true)
		expect(bodyFind('[data-testid="menu-form-hidden"]').exists()).toBe(true)
		expect(document.body.textContent).toContain('路由：必须以 / 开头，例如 /account/users')
		expect(document.body.textContent).toContain('页面路径：不能以 / 开头，页面文件为 web/src/views/<页面路径>/index.vue')
		expect(bodyGet('[data-testid="menu-form-component-path"]').text()).not.toContain('/reports')

		await bodyGet('[data-testid="menu-form-path"]').setValue('/reports')
		await bodyGet('[data-testid="menu-form-component-path"]').setValue('reports/orders')
    await bodyGet('[data-testid="menu-form-type"]').trigger('click')
    const actionOption = Array.from(document.body.querySelectorAll('.el-select-dropdown__item')).find((item) => item.textContent?.includes('按钮权限'))
    expect(actionOption).not.toBeUndefined()
    if (actionOption !== undefined) await new DOMWrapper(actionOption).trigger('click')
    await flushPromises()
		expect(bodyFind('[data-testid="menu-form-path"]').exists()).toBe(false)
		expect(bodyFind('[data-testid="menu-form-component-path"]').exists()).toBe(false)
		expect(bodyFind('[data-testid="menu-form-hidden"]').exists()).toBe(false)
  })

  it('keeps code readonly on edit and excludes it from the update payload', async () => {
    updateMenuMock.mockResolvedValue({ id: 2 })
    getMenusMock.mockResolvedValueOnce(menuTree()).mockResolvedValueOnce(menuTree())
    const wrapper = mountPage(pinia, ['rbac:menu:update'])
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
			name: '用户管理',
		i18nKey: 'navigation.accountUsers',
		path: '/account/users',
		componentPath: 'account/users',
      icon: 'lucide:panel-left',
      sortOrder: 12,
		isHidden: YesNo.No,
    }
		expect(updateMenuMock).toHaveBeenCalledWith(2, expected)
		expect(JSON.stringify(updateMenuMock.mock.calls[0]?.[1])).not.toContain('code')
  })

  it('filters parent options to valid nodes and excludes the edited subtree', async () => {
    getMenusMock.mockResolvedValue([rootWithEditableSubtree()])
    const wrapper = mountPage(pinia, ['rbac:menu:create', 'rbac:menu:update'])
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
    getMenusMock.mockResolvedValueOnce(mutableTree).mockResolvedValue(mutableTree)
    updateMenuStatusMock.mockResolvedValue({ id: 3, isEnabled: YesNo.Yes })
    deleteMenuMock.mockResolvedValue({ id: 3 })
    const wrapper = mountPage(pinia, ['rbac:menu:update', 'rbac:menu:delete'])
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
		expect(wrapper.get('[data-testid="menu-table"]').text()).toContain('用户管理')
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
		name: '系统管理',
    code: 'system',
    i18nKey: 'navigation.system',
    path: null,
		componentPath: null,
    icon: 'lucide:settings-2',
    sortOrder: 100,
    isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
		isProtected: YesNo.No,
    createdAt: timestamp,
    updatedAt: timestamp,
    children: [{
      id: 2,
      parentId: 1,
      menuType: 'page',
			name: '用户管理',
		code: 'account:user:list',
		i18nKey: 'navigation.accountUsers',
		path: '/account/users',
		componentPath: 'account/users',
    icon: 'lucide:panel-left',
      sortOrder: 10,
      isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
		isProtected: YesNo.No,
      createdAt: timestamp,
      updatedAt: timestamp,
      children: [{
        id: 3,
        parentId: 2,
        menuType: 'action',
				name: '修改用户',
		code: 'account:user:update',
		i18nKey: null,
        path: null,
		componentPath: null,
        icon: null,
        sortOrder: 10,
        isEnabled: YesNo.No,
		isHidden: YesNo.Yes,
		isProtected: YesNo.No,
        createdAt: timestamp,
        updatedAt: timestamp,
        children: [],
      }],
    }],
  }]
}

function protectedMenuTree(): ManagedMenuNode[] {
	const tree = menuTree()
	const root = tree[0]
	if (root === undefined || root.children[0] === undefined) throw new Error('test menu tree is incomplete')
	return [{
		...root,
		children: [{ ...root.children[0], isProtected: YesNo.Yes }],
	}]
}

function rootWithEditableSubtree(): ManagedMenuNode {
  const root = menuTree()[0]
  const directory: ManagedMenuNode = {
    id: 10,
    parentId: null,
    menuType: 'directory',
		name: '可编辑目录',
    code: 'reports',
    i18nKey: 'navigation.system',
    path: null,
		componentPath: null,
    icon: 'lucide:folder',
    sortOrder: 200,
    isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
		isProtected: YesNo.No,
    createdAt: root.createdAt,
    updatedAt: root.updatedAt,
    children: [{
      id: 11,
      parentId: 10,
      menuType: 'page',
			name: '可编辑页面',
      code: 'reports:list',
      i18nKey: 'navigation.accessMenus',
      path: '/reports',
		componentPath: 'account/users',
    icon: 'lucide:panel-left',
      sortOrder: 10,
      isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
		isProtected: YesNo.No,
      createdAt: root.createdAt,
      updatedAt: root.updatedAt,
      children: [],
    }],
  }
  return { ...root, children: [...root.children, directory] }
}
