import type { YesNo } from '@/enums/yesNo'
export type ConfigDialogMode = 'create' | 'edit' | 'version'
export interface ConfigForm {
  code: string
  name: string
  provider: number
  environment: number
  appId: string
  sellerId: string
  sortOrder: number
  isEnabled: YesNo
  notifyUrl: string
  payMethods: number[]
  privateKey: string
  appCertPEM: string
  alipayCertPEM: string
  rootCertPEM: string
}
