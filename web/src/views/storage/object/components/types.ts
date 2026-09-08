import type { YesNo } from '@/enums/yes-no'

export interface ConfigForm {
  name: string
  appId: string
  secretId: string
  secretKey: string
  bucket: string
  region: string
  endpoint: string | null
  bucketDomain: string | null
  isEnabled: YesNo
  remark: string
}

export interface RuleForm {
  platformId: number
  codes: string[]
  name: string
  cosConfigId: number
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  accessMode: 'private' | 'public'
  isEnabled: YesNo
  remark: string
}
