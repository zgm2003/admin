import { request } from '@/utils/request'

export interface UploadFileInput {
  fileName: string
  contentType: string
  fileSizeBytes: number
}

export interface UploadCredentialItem {
  fileId: number
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
  signal?: AbortSignal,
): Promise<UploadCredentialResponse> {
  const data = { ruleCode, files }
  return signal
    ? request.post<UploadCredentialResponse>('/api/admin/v1/storage/upload-credential', data, { signal })
    : request.post<UploadCredentialResponse>('/api/admin/v1/storage/upload-credential', data)
}

export async function requestObjectURL(
  objectKey: string,
  signal?: AbortSignal,
): Promise<ObjectURLResult> {
  return request.post<ObjectURLResult>(
    '/api/admin/v1/storage/object-url',
    { objectKey },
    { ...(signal ? { signal } : {}) },
  )
}
