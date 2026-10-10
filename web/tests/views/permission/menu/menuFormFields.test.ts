import { menuOptionsFixture } from './fixtures'
import { describe, expect, it } from 'vitest'
import {
  createMenuForm,
  createMenuInput,
  isMenuFormSubmittable,
} from '@/views/permission/menu/menuForm'

function pageForm() {
  return {
    ...createMenuForm(),
    menuType: 'page' as const,
    name: '用户',
    code: 'user:account:view',
    i18nKey: 'navigation.userAccount',
    path: '/user/account',
    componentPath: 'user/account',
  }
}

describe('menu form fields', () => {
  it('uses backend form constraints instead of a second local pattern and length catalog', () => {
    const constraints = { ...menuOptionsFixture.constraints, codePattern: '^.+$', nameMaxLength: 1 }
    expect(isMenuFormSubmittable(pageForm(), constraints)).toBe(false)
    expect(
      isMenuFormSubmittable({ ...pageForm(), name: 'A', code: 'BACKEND_NEW' }, constraints),
    ).toBe(true)
  })
  it('constructs explicit menu input from a submittable page form', () => {
    const form = pageForm()
    expect(isMenuFormSubmittable(form, menuOptionsFixture.constraints)).toBe(true)
    expect(createMenuInput(3, form)).toEqual({
      platformId: 3,
      parentId: null,
      menuType: 'page',
      name: '用户',
      code: 'user:account:view',
      i18nKey: 'navigation.userAccount',
      path: '/user/account',
      componentPath: 'user/account',
      icon: null,
      remark: null,
      sortOrder: 100,
      isEnabled: 1,
      isHidden: 0,
    })
  })

  it.each(['navigation.userAccount', 'reports.orders.list', 'permission.roleUpdate'])(
    'accepts i18n key %s for form submission',
    (i18nKey) => {
      expect(
        isMenuFormSubmittable({ ...pageForm(), i18nKey }, menuOptionsFixture.constraints),
      ).toBe(true)
    },
  )
  it.each([
    'navigation',
    'Navigation.users',
    'navigation.system_users',
    ' navigation.userAccount',
    'navigation.userAccount ',
  ])('blocks malformed i18n key %s in the form', (i18nKey) => {
    expect(isMenuFormSubmittable({ ...pageForm(), i18nKey }, menuOptionsFixture.constraints)).toBe(
      false,
    )
  })
  it.each([
    'system',
    'user:account:list',
    'permission:authPlatform:view',
    'permission:access-cache:rebuild',
    'message:mail:rate-limit:update',
  ])('accepts code format %s in the form', (code) => {
    expect(isMenuFormSubmittable({ ...pageForm(), code }, menuOptionsFixture.constraints)).toBe(
      true,
    )
  })
  it.each(['Permission:authPlatform:view', 'system:operation_log:list'])(
    'blocks malformed code %s in the form',
    (code) => {
      expect(isMenuFormSubmittable({ ...pageForm(), code }, menuOptionsFixture.constraints)).toBe(
        false,
      )
    },
  )
  it.each([
    '/user/account',
    '/permission/menu',
    '/reports/order-items',
    '/permission/authPlatform',
    '/system/operationLog',
  ])('accepts route path %s for form submission', (path) => {
    expect(isMenuFormSubmittable({ ...pageForm(), path }, menuOptionsFixture.constraints)).toBe(
      true,
    )
  })
  it.each([
    '/login',
    '/register',
    '/dashboard',
    'user/account',
    '/user/account/',
    '/system/:id',
    '/system//users',
    '/user/account?tab=1',
    '/user/account#top',
    '/system/operation_log',
  ])('blocks malformed route path %s in the form', (path) => {
    expect(isMenuFormSubmittable({ ...pageForm(), path }, menuOptionsFixture.constraints)).toBe(
      false,
    )
  })
  it.each([
    'user/account',
    'reports/order-items',
    'permission/authPlatform',
    'system/operationLog',
  ])('accepts component path %s in the form', (componentPath) => {
    expect(
      isMenuFormSubmittable({ ...pageForm(), componentPath }, menuOptionsFixture.constraints),
    ).toBe(true)
  })
  it.each([
    '/user/account',
    'user/account.vue',
    'user/account/',
    'system/:id',
    'system/../users',
    'system//users',
    'system/operation_log',
  ])('blocks malformed component path %s in the form', (componentPath) => {
    expect(
      isMenuFormSubmittable({ ...pageForm(), componentPath }, menuOptionsFixture.constraints),
    ).toBe(false)
  })
})
