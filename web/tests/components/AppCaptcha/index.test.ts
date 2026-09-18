import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCaptcha } from '@/api/auth/login'
import { appI18n, setLocale } from '@/i18n'
import AppCaptcha from '@/components/AppCaptcha/index.vue'

vi.mock('@/api/auth/login', () => ({ getCaptcha: vi.fn() }))
vi.mock('go-captcha-vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    Slide: defineComponent({
      name: 'GoCaptchaSlideTestDouble',
      props: {
        config: { type: Object, required: true },
        data: { type: Object, required: true },
        events: { type: Object, required: true },
      },
      template: '<div data-testid="go-captcha-slide" />',
    }),
  }
})

const getCaptchaMock = vi.mocked(getCaptcha)
const challenge = {
  captchaId: 'captcha-1',
  captchaType: 'slide' as const,
  masterImage: 'data:image/png;base64,master',
  tileImage: 'data:image/png;base64,tile',
  tileX: 24,
  tileY: 40,
  tileWidth: 48,
  tileHeight: 48,
  imageWidth: 300,
  imageHeight: 220,
  expiresIn: 120,
}

describe('AppCaptcha', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    getCaptchaMock.mockReset()
    getCaptchaMock.mockResolvedValue(challenge)
  })

  it('loads a challenge when opened and emits the completed proof', async () => {
    const wrapper = mountCaptcha()
    await flushPromises()

    expect(getCaptchaMock).toHaveBeenCalledTimes(1)
    const slide = wrapper.getComponent({ name: 'GoCaptchaSlideTestDouble' })
    expect(slide.props('data')).toEqual({
      thumbX: 24,
      thumbY: 40,
      thumbWidth: 48,
      thumbHeight: 48,
      image: 'data:image/png;base64,master',
      thumb: 'data:image/png;base64,tile',
    })

    const events = slide.props('events') as {
      confirm: (point: { x: number; y: number }) => void
    }
    events.confirm({
      x: 137.6,
      y: 40,
    })

    expect(wrapper.emitted('complete')).toEqual([
      [{ captchaId: 'captcha-1', captchaAnswer: { x: 138, y: 40 } }],
    ])
  })

  it('refreshes the challenge through the verifier control', async () => {
    const wrapper = mountCaptcha()
    await flushPromises()

    const events = wrapper.getComponent({ name: 'GoCaptchaSlideTestDouble' }).props('events') as {
      refresh: () => void
    }
    events.refresh()
    await flushPromises()

    expect(getCaptchaMock).toHaveBeenCalledTimes(2)
  })

  it('does not close from the backdrop while verification is pending', async () => {
    const wrapper = mountCaptcha({ loading: true })
    await flushPromises()

    await wrapper.get('[data-testid="app-captcha-overlay"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('offers an explicit retry when the challenge cannot be loaded', async () => {
    getCaptchaMock.mockRejectedValueOnce(new Error('network unavailable'))
    const wrapper = mountCaptcha()
    await flushPromises()

    expect(wrapper.get('[data-testid="app-captcha-retry"]').text()).toBe('换一张')

    await wrapper.get('[data-testid="app-captcha-retry"]').trigger('click')
    await flushPromises()

    expect(getCaptchaMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="go-captcha-slide"]').exists()).toBe(true)
  })
})

function mountCaptcha(props: { loading?: boolean } = {}) {
  return mount(AppCaptcha, {
    props: { modelValue: true, ...props },
    global: {
      plugins: [appI18n],
      stubs: {
        Teleport: true,
        ElIcon: { template: '<i><slot /></i>' },
      },
    },
  })
}
