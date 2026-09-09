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
  { view: 'user/loginLog', modules: ['user/loginLog'], label: 'navigation.userLoginLog' },
  {
    view: 'permission/authPlatform',
    modules: ['permission/authPlatform'],
    label: 'navigation.permissionAuthPlatform',
  },
  { view: 'permission/menu', modules: ['permission/menu'], label: 'navigation.permissionMenu' },
  { view: 'permission/role', modules: ['permission/role'], label: 'navigation.permissionRole' },
  {
    view: 'system/operationLog',
    modules: ['system/operationLog'],
    label: 'navigation.systemOperationLog',
  },
  {
    view: 'message/mail',
    modules: ['message/mail'],
    backendModules: [
      'message/mail/config',
      'message/mail/template',
      'message/mail/log',
      'message/mail/logVerification',
      'message/mail/rateLimitPolicy',
      'message/mail/recipientRule',
    ],
    viewModules: ['config', 'template', 'log', 'rateLimitPolicy', 'recipientRule'],
    label: 'navigation.mail',
  },
  {
    view: 'storage/object',
    modules: ['storage/cosConfig', 'storage/uploadRule'],
    label: 'navigation.storageObject',
  },
] as const

describe('cross-stack module identity', () => {
  for (const resource of resources) {
    it(resource.view, () => {
      expect(existsSync(resolve('src/views', resource.view, 'index.vue'))).toBe(true)
      if ('viewModules' in resource) {
        for (const module of resource.viewModules) {
          expect(existsSync(resolve('src/views', resource.view, module, 'index.vue'))).toBe(true)
        }
      }
      for (const module of resource.modules) {
        expect(existsSync(resolve('src/api', `${module}.ts`))).toBe(true)
      }
      const backendModules = 'backendModules' in resource ? resource.backendModules : resource.modules
      for (const module of backendModules) {
        expect(existsSync(resolve('../server/internal/module', module, 'model.go'))).toBe(true)
      }
      expect(Object.hasOwn(zhCN, resource.label)).toBe(true)
      expect(Object.hasOwn(enUS, resource.label)).toBe(true)
    })
  }
})
