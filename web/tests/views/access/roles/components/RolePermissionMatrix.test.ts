import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus, { ElCheckbox } from 'element-plus'
import { beforeEach, describe, expect, it } from 'vitest'

import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import type { RoleMatrixGroup } from '@src/views/access/roles/role-permission-matrix'
import RolePermissionMatrix from '@src/views/access/roles/components/RolePermissionMatrix.vue'

describe('RolePermissionMatrix', () => {
  beforeEach(() => {
    setLocale('zh-CN')
  })

  it('renders page and action columns with codes and disabled state', async () => {
    const wrapper = mountMatrix([2])
    await flushPromises()

    expect(wrapper.text()).toContain('页面权限')
    expect(wrapper.text()).toContain('操作权限')
    expect(wrapper.text()).toContain('角色管理')
    expect(wrapper.text()).toContain('rbac:role:list')
    expect(wrapper.text()).toContain('新增角色')
    expect(wrapper.text()).toContain('rbac:role:create')
    expect(wrapper.text()).toContain('已禁用')
  })

  it('reports a partially selected group', () => {
    const wrapper = mountMatrix([2])
    const groupCheckbox = checkboxContaining(wrapper, '系统管理')

    expect(groupCheckbox.props('modelValue')).toBe(false)
    expect(groupCheckbox.props('indeterminate')).toBe(true)
    expect(wrapper.text()).toContain('权限 1/2')
    expect(wrapper.text()).toContain('页面 1/1')
    expect(wrapper.text()).toContain('操作 0/1')
  })

  it('selects an action together with its page', async () => {
    const wrapper = mountMatrix([])
    await flushPromises()

    checkboxContaining(wrapper, '新增角色').vm.$emit('update:modelValue', true)
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[2, 3]])
  })

  it('clears a page and all of its actions', async () => {
    const wrapper = mountMatrix([2, 3])
    await flushPromises()

    checkboxContaining(wrapper, '角色管理').vm.$emit('update:modelValue', false)
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[]])
  })
})

function mountMatrix(modelValue: number[]) {
  return mount(RolePermissionMatrix, {
    props: {
      modelValue,
      groups: matrixGroups(),
    },
    global: {
      plugins: [appI18n, ElementPlus],
    },
  })
}

function checkboxContaining(
  wrapper: ReturnType<typeof mountMatrix>,
  text: string,
) {
  const checkbox = wrapper
    .findAllComponents(ElCheckbox)
    .find((candidate) => candidate.text().includes(text))
  if (checkbox === undefined) {
    throw new Error(`Unable to find checkbox containing ${text}`)
  }
  return checkbox
}

function matrixGroups(): RoleMatrixGroup[] {
  return [{
    groupKey: 'menu:1',
    groupCode: 'system',
		groupName: '系统管理',
    groupIsEnabled: YesNo.Yes,
    rows: [{
      pageId: 2,
      pageCode: 'rbac:role:list',
			pageName: '角色管理',
      pageIsEnabled: YesNo.Yes,
      actions: [{
        id: 3,
        code: 'rbac:role:create',
				name: '新增角色',
        isEnabled: YesNo.No,
      }],
    }],
  }]
}
