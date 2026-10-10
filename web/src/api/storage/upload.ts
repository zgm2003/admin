import { request } from '@/utils/request'

export interface UploadFileInput {
  fileName: string
  contentType: string
  fileSizeBytes: number
}

export interface UploadCredentialItem {
  uploadUrl: string
  objectKey: string
  method: 'PUT'
  headers: Record<string, string>
  expiresAt: string
  publicUrl?: string
}

export interface UploadCredentialResponse {
  items: UploadCredentialItem[]
}

export interface ObjectURLResult {
  url: string
  expiresAt: string | null
}

export async function requestUploadCredentials(
  ruleCode: string,
  files: UploadFileInput[],
): Promise<UploadCredentialResponse> {
  return request.post<UploadCredentialResponse>('/api/v1/storage/upload-credential', {
    ruleCode,
    files,
  })
}

export async function requestObjectURL(
  objectKey: string,
  signal?: AbortSignal,
): Promise<ObjectURLResult> {
  return request.post<ObjectURLResult>(
    '/api/v1/storage/object-url',
    { objectKey },
    { ...(signal ? { signal } : {}) },
  )
}
