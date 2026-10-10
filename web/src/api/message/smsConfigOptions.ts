import { request } from '@/utils/request'

export interface SmsConfigOptions {
  constraints: { minTTLMinutes: number; maxTTLMinutes: number }
  regions: Array<{ value: string; label: string }>
}

export function getSmsConfigOptions(): Promise<SmsConfigOptions> {
  return request.get<SmsConfigOptions>('/api/admin/v1/message/sms/config/options')
}
