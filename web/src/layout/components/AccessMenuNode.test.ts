import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { defineComponent, type PropType } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'

import type { AccessMenuNode as AccessMenuNodeDTO } from '../../api/access.contract'
import { menuIcons } from '../../access/menu-icons'
import { appI18n, setLocale } from '../../i18n'
import { pinia } from '../../store'
import { useAccessStore } from '../../store/access'
import AccessMenuNode from './AccessMenuNode.vue'
import AppAside from './AppAside.vue'

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

  it('renders the registered icon through menuIcons', () => {
    const wrapper = mountMenuNode(pageNode())

    expect(wrapper.findComponent(menuIcons.Setting).exists()).toBe(true)
  })

  it('updates the menu title from the active frontend locale', async () => {
    const wrapper = mountMenuNode(pageNode())
    expect(wrapper.text()).toContain('菜单管理')

    setLocale('en-US')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Menu management')
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
      props: { collapsed: false },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })
    const items = wrapper.findAllComponents({ name: 'ElMenuItem' })

    expect(items.map((item) => item.props('index'))).toEqual(['/dashboard', '/system/users'])
  })

  it('still shows Dashboard when the access tree is empty', () => {
    useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })

    const wrapper = mount(AppAside, {
      props: { collapsed: true },
      global: { plugins: [ElementPlus, pinia, createTestRouter(), appI18n] },
    })

    expect(wrapper.findAllComponents({ name: 'ElMenuItem' })).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'ElMenuItem' }).props('index')).toBe('/dashboard')
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
    viewKey: null,
    titleKey: 'navigation.system',
    icon: 'Folder',
    children: [pageNode()],
  }
}

function pageNode(): AccessMenuNodeDTO {
  return {
    code: 'system:user:list',
    menuType: 'page',
    path: '/system/users',
    viewKey: 'system-users',
    titleKey: 'navigation.systemMenus',
    icon: 'Setting',
    children: [],
  }
}
