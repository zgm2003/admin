import { request } from '@/utils/request'

export interface MailConfigOptions {
  constraints: { minTTLMinutes: number; maxTTLMinutes: number }
  regions: Array<{ value: string; label: string }>
}

export function getMailConfigOptions(): Promise<MailConfigOptions> {
  return request.get<MailConfigOptions>('/api/admin/v1/message/mail/config/options')
}
