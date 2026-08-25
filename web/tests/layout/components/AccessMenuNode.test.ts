import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { defineComponent, type PropType } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'

import type { AccessMenuNode as AccessMenuNodeDTO } from '@src/api/access.contract'
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

    expect(wrapper.findComponent({ name: 'ElSubMenu' }).props('index')).toBe('system')
    expect(wrapper.findAllComponents({ name: 'ElSubMenu' })).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'ElMenuItem' }).props('index')).toBe('/system/users')
  })

	it.each(['Setting', 'mdi:shield'])('passes icon name %s directly to DIcon', (icon) => {
		const node = pageNode()
		node.icon = icon
		const wrapper = mountMenuNode(node)
		expect(wrapper.findComponent(DIcon).props('icon')).toBe(icon)
	})

  it('updates the menu title from the active frontend locale', async () => {
    const wrapper = mountMenuNode(pageNode())
    expect(wrapper.text()).toContain('菜单管理')

    setLocale('en-US')
    await wrapper.vm.$nextTick()

		expect(wrapper.text()).toContain('Menu management')
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

  it('keeps Dashboard first and appends the access tree', () => {
    useAccessStore(pinia).applySnapshot({
      roleCodes: [],
      menuTree: [pageNode()],
      permissionCodes: [],
    })

    const wrapper = mount(AppAside, {
      props: { collapsed: false, uniqueOpened: true },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })
    const items = wrapper.findAllComponents({ name: 'ElMenuItem' })

    expect(items.map((item) => item.props('index'))).toEqual(['/dashboard', '/system/users'])
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

	it('shows static menu management after Dashboard only with its permission', () => {
		useAccessStore(pinia).applySnapshot({
			roleCodes: [], menuTree: [], permissionCodes: ['system:menu:list'],
		})
		const wrapper = mount(AppAside, {
			props: { collapsed: false, uniqueOpened: true },
			global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
		})
		expect(wrapper.findAllComponents({ name: 'ElMenuItem' }).map((item) => item.props('index')))
			.toEqual(['/dashboard', '/system/menus'])
	})

  it('passes the unique-opened preference to Element Plus menu', () => {
    const wrapper = mount(AppAside, {
      props: { collapsed: false, uniqueOpened: false },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })

    expect(wrapper.findComponent({ name: 'ElMenu' }).props('uniqueOpened')).toBe(false)
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
    code: 'system',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.system',
    icon: 'Folder',
		isHidden: YesNo.No,
    children: [pageNode()],
  }
}

function pageNode(): AccessMenuNodeDTO {
  return {
    code: 'system:user:list',
    menuType: 'page',
    path: '/system/users',
		componentPath: 'system/users',
		i18nKey: 'navigation.systemMenus',
    icon: 'Setting',
		isHidden: YesNo.No,
    children: [],
  }
}
