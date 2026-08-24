import { flushPromises, mount } from '@vue/test-utils'
import { House } from '@element-plus/icons-vue'
import { Icon } from '@iconify/vue'
import { describe, expect, it, vi } from 'vitest'
import DIcon from '@src/components/DIcon/src/index.vue'

describe('DIcon', () => {
  it('renders an explicit Element Plus component', () => {
    const wrapper = mount(DIcon, { props: { component: House } })
    expect(wrapper.find('[data-testid="d-icon"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(false)
  })

  it('renders an Iconify icon name', () => {
    const wrapper = mount(DIcon, { props: { icon: 'mdi:home' } })
    expect(wrapper.findComponent(Icon).exists()).toBe(true)
  })

  it('resolves an Element Plus icon name', async () => {
    const wrapper = mount(DIcon, { props: { icon: 'House' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(false)
  })

  it('shows an explicit empty state for invalid source props', () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const wrapper = mount(DIcon, { props: { icon: '', component: House } })
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(true)
    expect(error).toHaveBeenCalled()
    error.mockRestore()
  })
})
