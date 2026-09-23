import { h } from 'vue'
import { ElLink } from 'element-plus/es/components/link/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import type { ComposerTranslation } from 'vue-i18n'
import type { Router } from 'vue-router'

export interface LoginForm {
  account: string
  password: string
  code: string
}

export function isDigitChar(char: string): boolean {
  return /^\d$/.test(char)
}

export function safeRedirect(value: unknown): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/dashboard'
}

export function generateChallengeID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `c-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function showLoginSuccess(options: {
  isNewUser: boolean
  t: ComposerTranslation
  router: Router
}): void {
  const { isNewUser, t, router } = options
  if (!isNewUser) {
    ElNotification.success({ title: t('auth.login.success') })
    return
  }

  const profileLocation = router.resolve('/user/profile')
  ElNotification.success({
    title: t('auth.login.registeredSuccess'),
    message: h('span', [
      `${t('auth.login.registeredDescription')} `,
      h(
        ElLink,
        {
          type: 'primary',
          href: profileLocation.href,
          onClick: (event: MouseEvent) => {
            event.preventDefault()
            void router.push('/user/profile')
          },
        },
        () => t('auth.login.openProfile'),
      ),
    ]),
    duration: 8000,
  })
}
