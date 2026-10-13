import type { RequestClient } from '@/utils/request'
import type { CodeResult } from '@/api/user/profile'
export interface PhoneProof { target: 'current' | 'next'; phone?: string; challengeId?: string }
export interface PhoneBinding { currentChallengeId?: string; currentCode?: string; nextPhone: string; nextChallengeId: string; nextCode: string }
export const createPhoneApi = (client: RequestClient) => ({
  sendCode: (input: PhoneProof) => client.post<CodeResult, PhoneProof>('/api/v1/user/phone/send-code', input),
  bind: (input: PhoneBinding) => client.put<{ phone: string }, PhoneBinding>('/api/v1/user/phone', input),
})
