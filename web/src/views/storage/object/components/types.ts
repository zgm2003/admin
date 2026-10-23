import type { YesNo } from '@/enums/yesNo'

export interface ConfigForm {
  name: string
  appId: string
  secretId: string
  secretKey: string
  bucket: string
  region: string
  endpoint: string | null
  bucketDomain: string | null
  status: YesNo
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
  status: YesNo
  remark: string
}
