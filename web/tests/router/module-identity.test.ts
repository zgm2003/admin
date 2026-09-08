/// <reference types="node" />

import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import { enUS } from '@/i18n/messages/en-US'
import { zhCN } from '@/i18n/messages/zh-CN'

const resources = [
  { view: 'user/account', modules: ['user/account'], label: 'navigation.userAccount' },
  { view: 'user/profile', modules: ['user/profile'], label: 'layout.user.profile' },
  { view: 'user/session', modules: ['user/session'], label: 'navigation.userSession' },
  { view: 'user/loginlog', modules: ['user/loginlog'], label: 'navigation.userLoginlog' },
  {
    view: 'permission/authplatform',
    modules: ['permission/authplatform'],
    label: 'navigation.permissionAuthplatform',
  },
  { view: 'permission/menu', modules: ['permission/menu'], label: 'navigation.permissionMenu' },
  { view: 'permission/role', modules: ['permission/role'], label: 'navigation.permissionRole' },
  {
    view: 'system/operationlog',
    modules: ['system/operationlog'],
    label: 'navigation.systemOperationlog',
  },
  { view: 'message/mail', modules: ['message/mail'], label: 'navigation.mail' },
  {
    view: 'storage/object',
    modules: ['storage/cosconfig', 'storage/uploadrule'],
    label: 'navigation.storageObject',
  },
] as const

describe('cross-stack module identity', () => {
  for (const resource of resources) {
    it(resource.view, () => {
      expect(existsSync(resolve('src/views', resource.view, 'index.vue'))).toBe(true)
      for (const module of resource.modules) {
        expect(existsSync(resolve('src/api', `${module}.ts`))).toBe(true)
        expect(existsSync(resolve('../server/internal/module', module, 'model.go'))).toBe(true)
      }
      expect(Object.hasOwn(zhCN, resource.label)).toBe(true)
      expect(Object.hasOwn(enUS, resource.label)).toBe(true)
    })
  }
})
