import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { defineComponent, type PropType } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'

import type { AccessMenuNode as AccessMenuNodeDTO } from '@src/api/rbac/access'
import { DIcon } from '@src/components/DIcon'
import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import { pinia } from '@src/store'
import { useAccessStore } from '@src/store/access'
import AccessMenuNode from '@src/layout/components/AccessMenuNode.vue'
import AppAside from '@src/layout/components/AppAside.vue'

const MenuHarness = defineComponent({
  components: { AccessMenuNode },
  props: {
    node: { type: Object as PropType<AccessMenuNodeDTO>, required: true },
  },
  template: '<el-menu><AccessMenuNode :node="node" /></el-menu>',
})

describe('AccessMenuNode', () => {
  beforeEach(() => setLocale('zh-CN'))

  it('renders a directory as a submenu and its page child as a navigable item', () => {
    const wrapper = mountMenuNode(directoryNode())

    expect(wrapper.findComponent({ name: 'ElSubMenu' }).props('index')).toBe('account')
    expect(wrapper.findAllComponents({ name: 'ElSubMenu' })).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'ElMenuItem' }).props('index')).toBe('/account/users')
  })

	it.each(['lucide:settings-2', 'lucide:shield-check'])('passes local icon name %s directly to DIcon', (icon) => {
		const node = pageNode()
		node.icon = icon
		const wrapper = mountMenuNode(node)
		expect(wrapper.findComponent(DIcon).props('icon')).toBe(icon)
	})

  it('updates the menu title from the active frontend locale', async () => {
    const wrapper = mountMenuNode(pageNode())
    expect(wrapper.text()).toContain('用户管理')

    setLocale('en-US')
    await wrapper.vm.$nextTick()

		expect(wrapper.text()).toContain('User management')
	})

	it('renders a missing dynamic translation as its i18n key', () => {
		const node = pageNode()
		node.i18nKey = 'reports.orders.list'
		expect(mountMenuNode(node).text()).toContain('reports.orders.list')
	})

	it('does not render hidden pages or lift children from a hidden directory', () => {
		const hiddenPage = pageNode()
		hiddenPage.isHidden = YesNo.Yes
		expect(mountMenuNode(hiddenPage).findComponent({ name: 'ElMenuItem' }).exists()).toBe(false)

		const hiddenDirectory = directoryNode()
		hiddenDirectory.isHidden = YesNo.Yes
		const wrapper = mountMenuNode(hiddenDirectory)
		expect(wrapper.findComponent({ name: 'ElSubMenu' }).exists()).toBe(false)
		expect(wrapper.findComponent({ name: 'ElMenuItem' }).exists()).toBe(false)
	})
})

describe('AppAside access menu', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    useAccessStore(pinia).reset()
  })

  it('keeps Dashboard first and appends every access-tree root', () => {
    useAccessStore(pinia).applySnapshot({
      roleCodes: [],
			menuTree: navigationRoots(),
      permissionCodes: [],
    })

    const wrapper = mount(AppAside, {
      props: { collapsed: false, uniqueOpened: true },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })
    const items = wrapper.findAllComponents({ name: 'ElMenuItem' })

		expect(items.map((item) => item.props('index'))).toEqual([
			'/dashboard',
			'/account/users',
			'/access/roles',
			'/system/operation-logs',
		])
		expect(wrapper.findAllComponents({ name: 'ElSubMenu' }).map((item) => item.props('index')))
			.toEqual(['account', 'access', 'system'])
  })

	it('still shows Dashboard when the access tree is empty', () => {
    useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })

    const wrapper = mount(AppAside, {
      props: { collapsed: true, uniqueOpened: true },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
	})

		expect(wrapper.findAllComponents({ name: 'ElMenuItem' })).toHaveLength(1)
		expect(wrapper.findComponent({ name: 'ElMenuItem' }).props('index')).toBe('/dashboard')
	})

	it('passes the unique-opened preference to Element Plus menu', () => {
    const wrapper = mount(AppAside, {
      props: { collapsed: false, uniqueOpened: false },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })

		expect(wrapper.findComponent({ name: 'ElMenu' }).props('uniqueOpened')).toBe(false)
	})

  it('keeps the collapse transition enabled for the sidebar', () => {
		const wrapper = mount(AppAside, {
			props: { collapsed: true, uniqueOpened: true },
			global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
		})

		expect(wrapper.findComponent({ name: 'ElMenu' }).props('collapseTransition')).toBe(true)
	})

  it('renders the signed-in account in the sidebar footer and emits logout', () => {
    const wrapper = mount(AppAside, {
      props: {
        collapsed: false,
        uniqueOpened: true,
        username: 'admin',
        email: 'admin@example.com',
      },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })

    expect(wrapper.get('[data-testid="aside-account-name"]').text()).toBe('admin')
    wrapper.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'logout')

    expect(wrapper.emitted('logout')).toHaveLength(1)
  })
})

function mountMenuNode(node: AccessMenuNodeDTO) {
  return mount(MenuHarness, {
    props: { node },
    global: { plugins: [ElementPlus, appI18n] },
  })
}

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/dashboard', component: { template: '<div />' } }],
  })
}

function directoryNode(): AccessMenuNodeDTO {
  return {
		code: 'account',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.account',
    icon: 'lucide:folder',
		isHidden: YesNo.No,
    children: [pageNode()],
  }
}

function pageNode(): AccessMenuNodeDTO {
  return {
    code: 'account:user:list',
    menuType: 'page',
    path: '/account/users',
		componentPath: 'account/users',
		i18nKey: 'navigation.accountUsers',
    icon: 'lucide:settings-2',
		isHidden: YesNo.No,
    children: [],
  }
}

function navigationRoots(): AccessMenuNodeDTO[] {
	return [
		directoryWithPage('account', 'navigation.account', pageNode()),
		directoryWithPage('access', 'navigation.access', {
			...pageNode(),
			code: 'rbac:role:list',
			path: '/access/roles',
			componentPath: 'access/roles',
			i18nKey: 'navigation.accessRoles',
		}),
		directoryWithPage('system', 'navigation.system', {
			...pageNode(),
			code: 'audit:operation-log:list',
			path: '/system/operation-logs',
			componentPath: 'system/operation-logs',
			i18nKey: 'navigation.systemOperationLogs',
		}),
	]
}

function directoryWithPage(code: string, i18nKey: string, child: AccessMenuNodeDTO): AccessMenuNodeDTO {
	return {
		code,
		menuType: 'directory',
		path: null,
		componentPath: null,
		i18nKey,
		icon: 'lucide:folder',
		isHidden: YesNo.No,
		children: [child],
	}
}
