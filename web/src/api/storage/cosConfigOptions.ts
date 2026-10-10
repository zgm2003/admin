import { request } from '@/utils/request'

export interface CosConfigOptions {
  regions: Array<{ value: string; label: string }>
  extensions: Array<{ value: string; label: string }>
  mimeTypes: Array<{ value: string; label: string }>
}

export function getCosConfigOptions(): Promise<CosConfigOptions> {
  return request.get<CosConfigOptions>('/api/admin/v1/storage/cosconfig/options')
}
