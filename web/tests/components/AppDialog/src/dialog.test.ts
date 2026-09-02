import { describe, expect, it } from 'vitest'

import {
  filterAppDialogAttrs,
  resolveAppDialogAlignCenter,
  resolveAppDialogContentHeight,
  resolveAppDialogDraggable,
  resolveAppDialogWidth,
} from '@/components/AppDialog/dialog'

describe('AppDialog helpers', () => {
  it('resolves desktop and mobile sizing', () => {
    expect(resolveAppDialogWidth({ isMobile: false })).toBe('720px')
    expect(resolveAppDialogWidth({ isMobile: true })).toBe('94vw')
    expect(resolveAppDialogWidth({ isMobile: true, mobileWidth: 320 })).toBe('320px')
    expect(resolveAppDialogContentHeight(560)).toBe('560px')
    expect(resolveAppDialogContentHeight()).toBeUndefined()
  })

  it('filters fullscreen and disables desktop-only behaviors on mobile', () => {
    expect(filterAppDialogAttrs({ fullscreen: true, id: 'edit-user' })).toEqual({ id: 'edit-user' })
    expect(resolveAppDialogAlignCenter({ isMobile: true, alignCenter: true })).toBe(false)
    expect(resolveAppDialogDraggable({ isMobile: true, draggable: true })).toBe(false)
  })
})
