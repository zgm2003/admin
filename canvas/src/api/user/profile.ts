import type { RequestClient } from '@/utils/request'

export interface Profile { userId: number; username: string; email: string; phone: string | null; avatar: string; birthday: string | null; gender: number }
export interface ProfileInput { username: string; birthday: string | null; gender: number; avatar: string; avatarFileId?: number; avatarRequestKey?: string }
export interface ProfileOptions { genders: { value: number; label: string }[] }
export interface PasswordInput { currentPassword: string; newPassword: string; confirmPassword: string }
export interface PasswordCodeInput { loginType: 'email' | 'phone'; challengeId: string; code: string; newPassword: string; confirmPassword: string }
export interface CodeResult { challengeId: string; expiresAt: string; resendAfterSeconds: number }
export function createProfileApi(client: RequestClient) {
  return {
    get: (signal?: AbortSignal) => client.get<Profile>('/api/canvas/v1/user/profile', { signal }),
    options: (signal?: AbortSignal) => client.get<ProfileOptions>('/api/canvas/v1/user/profile/options', { signal }),
    update: (input: ProfileInput, signal?: AbortSignal) => client.put<Profile & { updatedAt: string }, ProfileInput>('/api/canvas/v1/user/profile', input, { signal }),
    changePassword: (input: PasswordInput) => client.post<void, PasswordInput>('/api/canvas/v1/user/password', input),
    setPassword: (input: Omit<PasswordInput, 'currentPassword'>) => client.post<void>('/api/canvas/v1/user/password/set', input),
    sendPasswordCode: (loginType: 'email' | 'phone') => client.post<CodeResult>('/api/canvas/v1/user/password/send-code', { loginType }),
    changePasswordByCode: (input: PasswordCodeInput) => client.put<void, PasswordCodeInput>('/api/canvas/v1/user/password/by-code', input),
  }
}
