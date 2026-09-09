import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { defineComponent, type PropType } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'

import type { PermissionMenuNode as PermissionMenuNodeDTO } from '@/api/permission/permission'
import { AppDIcon } from '@/components/AppDIcon'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import PermissionMenuNode from '@/layout/components/PermissionMenuNode/index.vue'
import AppAside from '@/layout/components/AppAside/index.vue'

const MenuHarness = defineComponent({
  components: { PermissionMenuNode },
  props: {
    node: { type: Object as PropType<PermissionMenuNodeDTO>, required: true },
  },
  template: '<el-menu><PermissionMenuNode :node="node" /></el-menu>',
})

describe('PermissionMenuNode', () => {
  beforeEach(() => setLocale('zh-CN'))

  it('renders a directory as a submenu and its page child as a navigable item', () => {
    const wrapper = mountMenuNode(directoryNode())

    expect(wrapper.findComponent({ name: 'ElSubMenu' }).props('index')).toBe('account')
    expect(wrapper.findAllComponents({ name: 'ElSubMenu' })).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'ElMenuItem' }).props('index')).toBe('/user/account')
  })

  it.each(['lucide:settings-2', 'lucide:shield-check'])(
    'passes local icon name %s directly to AppDIcon',
    (icon) => {
      const node = pageNode()
      node.icon = icon
      const wrapper = mountMenuNode(node)
      expect(wrapper.findComponent(AppDIcon).props('icon')).toBe(icon)
    },
  )

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
    usePermissionStore(pinia).reset()
  })

  it('keeps Dashboard first and appends every access-tree root', () => {
    usePermissionStore(pinia).applySnapshot({
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
      '/user/account',
      '/permission/role',
      '/system/operationLog',
    ])
    expect(
      wrapper.findAllComponents({ name: 'ElSubMenu' }).map((item) => item.props('index')),
    ).toEqual(['account', 'access', 'system'])
  })

  it('still shows Dashboard when the access tree is empty', () => {
    usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })

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

  it('disables the EP collapse content transition so width animates without clipped labels', () => {
    const wrapper = mount(AppAside, {
      props: { collapsed: true, uniqueOpened: true },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })

    expect(wrapper.findComponent({ name: 'ElMenu' }).props('collapseTransition')).toBe(false)
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

function mountMenuNode(node: PermissionMenuNodeDTO) {
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

function directoryNode(): PermissionMenuNodeDTO {
  return {
    code: 'account',
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey: 'navigation.user',
    icon: 'lucide:folder',
    isHidden: YesNo.No,
    children: [pageNode()],
  }
}

function pageNode(): PermissionMenuNodeDTO {
  return {
    code: 'user:account:list',
    menuType: 'page',
    path: '/user/account',
    componentPath: 'user/account',
    i18nKey: 'navigation.userAccount',
    icon: 'lucide:settings-2',
    isHidden: YesNo.No,
    children: [],
  }
}

function navigationRoots(): PermissionMenuNodeDTO[] {
  return [
    directoryWithPage('account', 'navigation.user', pageNode()),
    directoryWithPage('access', 'navigation.permission', {
      ...pageNode(),
      code: 'permission:role:list',
      path: '/permission/role',
      componentPath: 'permission/role',
      i18nKey: 'navigation.permissionRole',
    }),
    directoryWithPage('system', 'navigation.system', {
      ...pageNode(),
      code: 'system:operationLog:list',
      path: '/system/operationLog',
      componentPath: 'system/operationLog',
      i18nKey: 'navigation.systemOperationLog',
    }),
  ]
}

function directoryWithPage(
  code: string,
  i18nKey: string,
  child: PermissionMenuNodeDTO,
): PermissionMenuNodeDTO {
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
