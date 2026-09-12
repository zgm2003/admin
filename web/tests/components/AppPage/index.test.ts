import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import { AppPage } from '@/components/AppPage'

describe('AppPage', () => {
  it('provides the shared management shell and forwards page attributes', () => {
    const wrapper = mount(AppPage, {
      attrs: { class: 'dictionary-page', 'aria-label': 'Dictionaries', 'data-testid': 'page' },
      slots: { default: '<span>content</span>' },
    })

    expect(wrapper.element.tagName).toBe('SECTION')
    expect(wrapper.classes()).toEqual(
      expect.arrayContaining(['app-page', 'management-page', 'dictionary-page']),
    )
    expect(wrapper.attributes('aria-label')).toBe('Dictionaries')
    expect(wrapper.attributes('data-testid')).toBe('page')
    expect(wrapper.text()).toBe('content')
  })
})
