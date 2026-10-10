import { request } from '@/utils/request'

export interface UserProfileOptions {
  genders: Array<{ value: number; label: string }>
}

export function getUserProfileOptions(): Promise<UserProfileOptions> {
  return request.get<UserProfileOptions>('/api/admin/v1/user/profile/options')
}
