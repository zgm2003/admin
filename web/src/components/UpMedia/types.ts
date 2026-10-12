export interface UpMediaProps {
  modelValue: string | string[]
  ruleCode: string
  multiple?: boolean
  variant?: 'default' | 'avatar' | 'file'
  fileLabel?: string
  uploadDisabled?: boolean
  /** Show the personal file library entry after the caller checks its permissions. */
  allowFileLibrary?: boolean
  /** Pass the field's backend selection purpose, for example avatar. */
  fileLibraryPurpose?: string
  accept?: string
  disabled?: boolean
  clearable?: boolean
  width?: string
}

export interface PreviewState {
  url: string
  expiresAt: string | null
  requestId: number
  pending: boolean
}

export interface MediaFileItem {
  objectKey: string
  previewUrl: string
  pending: boolean
  failed: boolean
}
