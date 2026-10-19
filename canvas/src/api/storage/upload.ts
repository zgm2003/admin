import type { RequestClient } from '@/utils/request'
export interface UploadCredential { fileId: number; uploadUrl: string; objectKey: string; method: 'PUT'; headers: Record<string, string>; expiresAt: string; publicUrl?: string }
export interface CompletedFile { fileId: number; objectKey: string; publicUrl?: string }
export function createUploadApi(client: RequestClient) {
  return {
    credentials: (file: File, signal?: AbortSignal) => client.post<{ items: UploadCredential[] }>('/api/canvas/v1/storage/upload-credential', { ruleCode: 'avatar', files: [{ fileName: file.name, contentType: file.type, fileSizeBytes: file.size }] }, { signal }),
    complete: (fileId: number, signal?: AbortSignal) => client.post<CompletedFile>(`/api/canvas/v1/storage/file/${fileId}/complete`, undefined, { signal }),
    objectURL: (objectKey: string, signal?: AbortSignal) => client.post<{ url: string; expiresAt: string | null }>('/api/canvas/v1/storage/object-url', { objectKey }, { signal }),
  }
}
