import { mount } from '@vue/test-utils'
import { House } from '@element-plus/icons-vue'
import { describe, expect, it, vi } from 'vitest'
import DIcon from '@src/components/DIcon/src/index.vue'

describe('DIcon', () => {
  it('renders an explicit Element Plus component', () => {
    const wrapper = mount(DIcon, { props: { component: House } })
    expect(wrapper.find('[data-testid="d-icon"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(false)
  })

  it('renders a registered local Lucide icon without fetching', () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(() => {
      throw new Error('network access is forbidden')
    })
    const wrapper = mount(DIcon, { props: { icon: 'lucide:house' } })
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(fetch).not.toHaveBeenCalled()
    fetch.mockRestore()
  })

  it('shows an explicit empty state for an invalid Lucide source', () => {
    const wrapper = mount(DIcon, { props: { icon: 'lucide:not-in-registry' as never } })
    expect(wrapper.find('[data-testid="d-icon-empty"]').exists()).toBe(true)
  })

})
