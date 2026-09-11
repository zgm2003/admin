import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@/i18n'
import * as profileAPI from '@/api/user/profile'
import * as emailAPI from '@/api/user/email'
import * as phoneAPI from '@/api/user/phone'
import { getDictionaryOptions } from '@/api/system/dictionary'
import ProfilePage from '@/views/user/profile/index.vue'
import UpMedia from '@/components/UpMedia/index.vue'
import ProfileHero from '@/views/user/profile/components/ProfileHero/index.vue'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'

vi.mock('@/api/user/profile', () => ({
  getAccountProfile: vi.fn(),
  updateAccountProfile: vi.fn(),
  changePassword: vi.fn(),
  setPassword: vi.fn(),
  sendPasswordCode: vi.fn(),
  changePasswordByCode: vi.fn(),
}))
vi.mock('@/api/user/email', () => ({ sendEmailCode: vi.fn(), bindEmail: vi.fn() }))
vi.mock('@/api/user/phone', () => ({ sendPhoneCode: vi.fn(), bindPhone: vi.fn() }))
vi.mock('@/api/system/dictionary', () => ({ getDictionaryOptions: vi.fn() }))

const getAccountProfile = vi.mocked(profileAPI.getAccountProfile)
const updateAccountProfile = vi.mocked(profileAPI.updateAccountProfile)
const changePassword = vi.mocked(profileAPI.changePassword)
const setPassword = vi.mocked(profileAPI.setPassword)
const sendPasswordCode = vi.mocked(profileAPI.sendPasswordCode)
const changePasswordByCode = vi.mocked(profileAPI.changePasswordByCode)
const sendEmailCode = vi.mocked(emailAPI.sendEmailCode)
const bindEmail = vi.mocked(emailAPI.bindEmail)
const sendPhoneCode = vi.mocked(phoneAPI.sendPhoneCode)
const bindPhone = vi.mocked(phoneAPI.bindPhone)
const getDictionaryOptionsMock = vi.mocked(getDictionaryOptions)
const mountedWrappers: VueWrapper[] = []

describe('account profile permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    getDictionaryOptionsMock.mockResolvedValue({
      'user.gender': [
        { label: '未知', value: '0' },
        { label: '男', value: '1' },
        { label: '女', value: '2' },
      ],
    })
    getAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: '',
      birthday: null,
      gender: 0,
    })
    sendEmailCode.mockResolvedValue({
      challengeId: 'email-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    bindEmail.mockResolvedValue({ email: 'next@example.com' })
    sendPhoneCode.mockResolvedValue({
      challengeId: 'phone-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    bindPhone.mockResolvedValue({ phone: '+8615671628271' })
    sendPasswordCode.mockResolvedValue({
      challengeId: 'password-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    changePasswordByCode.mockResolvedValue(undefined)
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it.each([
    { permissions: ['user:profile:update'], save: true, password: false },
    { permissions: ['user:password:update'], save: false, password: true },
    { permissions: [], save: false, password: false },
  ])('shows only actions granted by $permissions', async ({ permissions, save, password }) => {
    const wrapper = mountPage(permissions)
    await flushPromises()

    expect(wrapper.find('[data-testid="account-profile-save"]').exists()).toBe(save)
    expect(wrapper.find('[data-testid="account-password-submit"]').exists()).toBe(password)
  })

  it('loads localized gender options from the system dictionary and preserves numeric values', async () => {
    const wrapper = mountPage([])
    await flushPromises()

    expect(getDictionaryOptionsMock).toHaveBeenCalledWith(['user.gender'])
    const genderSelect = wrapper.getComponent({ name: 'ElSelectV2' })
    expect(genderSelect.attributes('data-testid')).toBe('account-profile-gender')
    expect(genderSelect.props('options')).toEqual([
      { label: '未知', value: 0 },
      { label: '男', value: 1 },
      { label: '女', value: 2 },
    ])
  })

  it('does not substitute hardcoded gender options when the dictionary fails', async () => {
    getDictionaryOptionsMock.mockRejectedValueOnce(new Error('dictionary unavailable'))
    const wrapper = mountPage([])
    await flushPromises()

    const genderSelect = wrapper.getComponent({ name: 'ElSelectV2' })
    expect(genderSelect.props('options')).toEqual([])
    expect(genderSelect.props('disabled')).toBe(true)
    expect(wrapper.text()).toContain('性别选项加载失败')
  })

  it('rejects malformed gender dictionary values instead of coercing them', async () => {
    getDictionaryOptionsMock.mockResolvedValueOnce({
      'user.gender': [{ label: '其他', value: 'female' }],
    })
    const wrapper = mountPage([])
    await flushPromises()

    expect(wrapper.getComponent({ name: 'ElSelectV2' }).props('options')).toEqual([])
    expect(wrapper.text()).toContain('性别选项加载失败')
  })

  it('reloads gender labels when the active language changes', async () => {
    getDictionaryOptionsMock
      .mockReset()
      .mockResolvedValueOnce({
        'user.gender': [
          { label: '未知', value: '0' },
          { label: '男', value: '1' },
          { label: '女', value: '2' },
        ],
      })
      .mockResolvedValueOnce({
        'user.gender': [
          { label: 'Unknown', value: '0' },
          { label: 'Male', value: '1' },
          { label: 'Female', value: '2' },
        ],
      })
    const wrapper = mountPage([])
    await flushPromises()

    setLocale('en-US')
    await flushPromises()

    expect(getDictionaryOptionsMock).toHaveBeenCalledTimes(2)
    expect(wrapper.getComponent({ name: 'ElSelectV2' }).props('options')).toEqual([
      { label: 'Unknown', value: 0 },
      { label: 'Male', value: 1 },
      { label: 'Female', value: 2 },
    ])
  })

  it('does not emit a second error toast when saving the profile fails', async () => {
    updateAccountProfile.mockRejectedValue(new Error('保存失败'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['user:profile:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('loads the avatar object key and submits it with profile changes', async () => {
    getAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: 'avatar/old.png',
      birthday: null,
      gender: 0,
    })
    updateAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: 'avatar/new.png',
      birthday: null,
      gender: 0,
      updatedAt: '2026-08-30T00:00:00Z',
    })
    const wrapper = mountPage(['user:profile:update'])
    await flushPromises()

    const media = wrapper.findComponent(UpMedia)
    expect(media.props('modelValue')).toBe('avatar/old.png')
    media.vm.$emit('preview-change', 'https://cdn.example/avatar/old.png')
    await flushPromises()
    expect(wrapper.getComponent(ProfileHero).props('avatarUrl')).toBe(
      'https://cdn.example/avatar/old.png',
    )
    media.vm.$emit('update:modelValue', 'avatar/new.png')
    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(updateAccountProfile).toHaveBeenCalledWith(
      expect.objectContaining({ avatar: 'avatar/new.png' }),
    )
    expect(useAuthStore().user?.avatar).toBe('avatar/new.png')
  })

  it('does not emit a second error toast when changing password fails', async () => {
    changePassword.mockRejectedValue(new Error('密码错误'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['user:password:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-password-submit"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('sets the first password without asking for the current one', async () => {
    setPassword.mockResolvedValue(undefined)
    const wrapper = mountPage(['user:password:update'], true)
    await flushPromises()

    expect(wrapper.text()).toContain('设置密码')
    expect(wrapper.find('[data-testid="account-password-current"]').exists()).toBe(false)

    await wrapper.get('[data-testid="account-password-new"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="account-password-confirm"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="account-password-submit"]').trigger('click')
    await flushPromises()

    expect(setPassword).toHaveBeenCalledWith({
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    })
    expect(changePassword).not.toHaveBeenCalled()
    expect(useAuthStore().passwordSetRequired).toBe(false)
  })

  it('shows independent email and phone identity actions only with exact permissions', async () => {
    const emailOnly = mountPage(['user:email:update'])
    await flushPromises()
    expect(emailOnly.find('[data-testid="profile-email-action"]').exists()).toBe(true)
    expect(emailOnly.find('[data-testid="profile-phone-action"]').exists()).toBe(false)
    emailOnly.unmount()

    const phoneOnly = mountPage(['user:phone:update'])
    await flushPromises()
    expect(phoneOnly.find('[data-testid="profile-email-action"]').exists()).toBe(false)
    expect(phoneOnly.find('[data-testid="profile-phone-action"]').exists()).toBe(true)
  })

  it('binds a first email using only the next bind_email proof', async () => {
    getAccountProfile.mockResolvedValueOnce({
      userId: 7,
      username: 'alice',
      email: '',
      phone: '+8613800000000',
      avatar: '',
      birthday: null,
      gender: 0,
    })
    const wrapper = mountPage(['user:email:update'], false, { email: '', phone: '+8613800000000' })
    await flushPromises()

    await wrapper.get('[data-testid="profile-email-action"]').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-testid="profile-email-current-send"]')).toBeNull()
    await setBodyInput('profile-email-next', 'next@example.com')
    await clickBodyTestID('profile-email-next-send')
    await setBodyInput('profile-email-next-code', '222222')
    await clickBodyTestID('profile-email-submit')
    await flushPromises()

    expect(sendEmailCode).toHaveBeenCalledWith({ target: 'next', email: 'next@example.com' })
    expect(bindEmail).toHaveBeenCalledWith({
      nextEmail: 'next@example.com',
      nextChallengeId: 'email-challenge',
      nextCode: '222222',
    })
    expect(useAuthStore().user?.email).toBe('next@example.com')
    expect(useAuthStore().user?.phone).toBe('+8613800000000')
  })

  it('changes a phone only after current and next bind_phone proofs', async () => {
    getAccountProfile.mockResolvedValueOnce({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: '+8613800000000',
      avatar: '',
      birthday: null,
      gender: 0,
    })
    const wrapper = mountPage(['user:phone:update'], false, {
      email: 'alice@example.com',
      phone: '+8613800000000',
    })
    await flushPromises()

    await wrapper.get('[data-testid="profile-phone-action"]').trigger('click')
    await flushPromises()
    await clickBodyTestID('profile-phone-current-send')
    await setBodyInput('profile-phone-current-code', '111111')
    await setBodyInput('profile-phone-next', '15671628271')
    await clickBodyTestID('profile-phone-next-send')
    await setBodyInput('profile-phone-next-code', '222222')
    await clickBodyTestID('profile-phone-submit')
    await flushPromises()

    expect(sendPhoneCode).toHaveBeenNthCalledWith(1, { target: 'current' })
    expect(sendPhoneCode).toHaveBeenNthCalledWith(2, { target: 'next', phone: '15671628271' })
    expect(bindPhone).toHaveBeenCalledWith({
      currentChallengeId: 'phone-challenge',
      currentCode: '111111',
      nextPhone: '15671628271',
      nextChallengeId: 'phone-challenge',
      nextCode: '222222',
    })
    expect(useAuthStore().user?.email).toBe('alice@example.com')
    expect(useAuthStore().user?.phone).toBe('+8615671628271')
  })

  it.each(['email', 'phone'] as const)(
    'changes password with the current %s identity and keeps the active session',
    async (loginType) => {
      getAccountProfile.mockResolvedValueOnce({
        userId: 7,
        username: 'alice',
        email: 'alice@example.com',
        phone: '+8613800000000',
        avatar: '',
        birthday: null,
        gender: 0,
      })
      const wrapper = mountPage(['user:password:update'], false, {
        email: 'alice@example.com',
        phone: '+8613800000000',
      })
      await flushPromises()
      await wrapper.get('[data-testid="account-profile-tab-security"]').trigger('click')
      const mode = wrapper.getComponent({ name: 'ElSegmented' })
      mode.vm.$emit('update:modelValue', 'code')
      await flushPromises()
      const identityType = wrapper.findAllComponents({ name: 'ElSegmented' })[1]
      identityType.vm.$emit('update:modelValue', loginType)
      await clickTestID(wrapper, 'account-password-code-send')
      await wrapper.get('[data-testid="account-password-code"]').setValue('123456')
      await wrapper.get('[data-testid="account-password-new"]').setValue('NewPassw0rd!')
      await wrapper.get('[data-testid="account-password-confirm"]').setValue('NewPassw0rd!')
      await clickTestID(wrapper, 'account-password-submit')
      await flushPromises()

      expect(sendPasswordCode).toHaveBeenCalledWith(loginType)
      expect(changePasswordByCode).toHaveBeenCalledWith({
        loginType,
        challengeId: 'password-challenge',
        code: '123456',
        newPassword: 'NewPassw0rd!',
        confirmPassword: 'NewPassw0rd!',
      })
      expect(useAuthStore().status).toBe('authenticated')
    },
  )
})

function mountPage(
  permissionCodes: string[],
  passwordSetRequired = false,
  identities: { email: string; phone: string | null } = {
    email: 'alice@example.com',
    phone: null,
  },
) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  useAuthStore(pinia).setAuthenticated({
    userId: 7,
    username: 'alice',
    email: identities.email,
    phone: identities.phone,
    avatar: '',
    passwordSetRequired,
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/login', name: 'login', component: { template: '<div />' } }],
  })
  const wrapper = mount(ProfilePage, {
    global: { plugins: [pinia, appI18n, ElementPlus, router] },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

async function setBodyInput(testID: string, value: string): Promise<void> {
  const input = document.body.querySelector<HTMLInputElement>(
    `[data-testid="${testID}"] input, input[data-testid="${testID}"]`,
  )
  if (input === null) throw new Error(`${testID} input missing`)
  input.value = value
  input.dispatchEvent(new Event('input'))
  await Promise.resolve()
}

async function clickBodyTestID(testID: string): Promise<void> {
  const button = document.body.querySelector<HTMLButtonElement>(`[data-testid="${testID}"]`)
  if (button === null) throw new Error(`${testID} button missing`)
  button.click()
  await Promise.resolve()
}

async function clickTestID(wrapper: VueWrapper, testID: string): Promise<void> {
  await wrapper.get(`[data-testid="${testID}"]`).trigger('click')
}
