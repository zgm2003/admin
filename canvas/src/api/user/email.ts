import type { RequestClient } from '@/utils/request'
import type { CodeResult } from '@/api/user/profile'
export interface EmailProof { target: 'current' | 'next'; email?: string; challengeId?: string }
export interface EmailBinding { currentChallengeId?: string; currentCode?: string; nextEmail: string; nextChallengeId: string; nextCode: string }
export const createEmailApi = (client: RequestClient) => ({
  sendCode: (input: EmailProof) => client.post<CodeResult, EmailProof>('/api/canvas/v1/user/email/send-code', input),
  bind: (input: EmailBinding) => client.put<{ email: string }, EmailBinding>('/api/canvas/v1/user/email', input),
})
