import { request } from '../../utils/request'

export interface UploadFileInput { fileName: string; contentType: string; fileSizeBytes: number }
export interface UploadCredentialItem { uploadUrl: string; objectKey: string; method: 'PUT'; headers: Record<string, string>; expiresAt: string; publicUrl?: string }
export interface UploadCredentialResponse { items: UploadCredentialItem[] }

export function requestUploadCredentials(ruleCode: string, files: UploadFileInput[]): Promise<UploadCredentialResponse> {
  return request<UploadCredentialResponse>({ method: 'POST', url: '/api/v1/storage/upload-credentials', data: { ruleCode, files } })
}

export function requestObjectURL(ruleCode: string, objectKey: string): Promise<{ url: string }> {
  return request<{ url: string }>({ method: 'POST', url: '/api/v1/storage/object-url', data: { ruleCode, objectKey } })
}
