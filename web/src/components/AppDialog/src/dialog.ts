export const DEFAULT_APP_DIALOG_WIDTH = '720px'
export const DEFAULT_APP_DIALOG_MOBILE_WIDTH = '94vw'
export const DEFAULT_APP_DIALOG_BODY_PADDING = '20px'
export const DEFAULT_APP_DIALOG_MOBILE_BODY_PADDING = '12px 16px'

export type AppDialogSize = string | number

export function toCssLength(value?: AppDialogSize): string | undefined {
  if (typeof value === 'number') return `${value}px`
  if (typeof value === 'string' && value.trim() !== '') return value
  return undefined
}

export function resolveAppDialogWidth(params: {
  isMobile: boolean
  width?: AppDialogSize
  mobileWidth?: AppDialogSize
}): string {
  return params.isMobile
    ? toCssLength(params.mobileWidth) ?? DEFAULT_APP_DIALOG_MOBILE_WIDTH
    : toCssLength(params.width) ?? DEFAULT_APP_DIALOG_WIDTH
}

export function resolveAppDialogContentHeight(height?: AppDialogSize): string | undefined {
  return toCssLength(height)
}

export function resolveAppDialogBodyPadding(params: {
  isMobile: boolean
  bodyPadding?: AppDialogSize
}): string {
  return toCssLength(params.bodyPadding)
    ?? (params.isMobile ? DEFAULT_APP_DIALOG_MOBILE_BODY_PADDING : DEFAULT_APP_DIALOG_BODY_PADDING)
}

export function resolveAppDialogPadding(padding?: AppDialogSize): string | undefined {
  return toCssLength(padding)
}

export function resolveAppDialogAlignCenter(params: {
  isMobile: boolean
  alignCenter?: boolean
}): boolean {
  return params.isMobile ? false : Boolean(params.alignCenter)
}

export function resolveAppDialogDraggable(params: {
  isMobile: boolean
  draggable?: boolean
}): boolean {
  return params.isMobile ? false : params.draggable ?? true
}

export function filterAppDialogAttrs(attrs: Record<string, unknown>): Record<string, unknown> {
  const filtered = { ...attrs }
  delete filtered.fullscreen
  return filtered
}
