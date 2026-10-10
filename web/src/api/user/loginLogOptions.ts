import { request } from '@/utils/request'

export interface LoginLogOptions {
  eventTypes: Array<{ value: number; label: string }>
  loginTypes: Array<{ value: number; label: string }>
}

export function getLoginLogOptions(): Promise<LoginLogOptions> {
  return request.get<LoginLogOptions>('/api/admin/v1/user/loginlog/options')
}
