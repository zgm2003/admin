import { mount } from '@vue/test-utils'
import { House } from '@element-plus/icons-vue'
import { describe, expect, it, vi } from 'vitest'
import AppDIcon from '@src/components/AppDIcon/src/index.vue'

describe('AppDIcon', () => {
  it('renders an explicit Element Plus component', () => {
    const wrapper = mount(AppDIcon, { props: { component: House } })
    expect(wrapper.find('[data-testid="d-icon"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="d-icon"]').element.tagName).toBe('I')
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(false)
  })

  it('renders a registered local Lucide icon without fetching', () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(() => {
      throw new Error('network access is forbidden')
    })
    const wrapper = mount(AppDIcon, { props: { icon: 'lucide:house' } })
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(fetch).not.toHaveBeenCalled()
    fetch.mockRestore()
  })

  it('shows an explicit empty state for an invalid Lucide source', () => {
    const wrapper = mount(AppDIcon, { props: { icon: 'lucide:not-in-registry' as never } })
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(true)
  })

})
