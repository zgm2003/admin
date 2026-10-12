import { request } from '@/utils/request'
import type { PageResult } from '@/types/pagination'
import type { BackendOption } from '@/types/option'

export interface StorageFile {
  id: number
  fileName: string
  mimeType: string
  sizeBytes: number
  category: string
  createdAt: string
  objectKey: string
}
export interface FileListInput {
  page: number
  pageSize: number
  keyword?: string
  category?: string
  purpose?: string
}
export interface FileUploadRule extends BackendOption<string> {
  accept: string
  maxFileSizeBytes: number
}
export interface FileOptions {
  categories: BackendOption<string>[]
  uploadRules: FileUploadRule[]
}
export interface CompletedFile {
  fileId: number
  objectKey: string
  publicUrl?: string
}
export interface FileURL {
  url: string
  expiresAt: string
}

export function listFiles(
  input: FileListInput,
  signal?: AbortSignal,
): Promise<PageResult<StorageFile>> {
  return request.get<PageResult<StorageFile>>('/api/v1/storage/file', {
    params: input,
    ...(signal ? { signal } : {}),
  })
}
export function getFileOptions(purpose = '', signal?: AbortSignal): Promise<FileOptions> {
  return request.get<FileOptions>('/api/v1/storage/file/options', {
    params: { purpose },
    ...(signal ? { signal } : {}),
  })
}
export function getFile(id: number): Promise<StorageFile> {
  return request.get<StorageFile>(`/api/v1/storage/file/${id}`)
}
export function completeFileUpload(id: number, signal?: AbortSignal): Promise<CompletedFile> {
  return request.post<CompletedFile>(
    `/api/v1/storage/file/${id}/complete`,
    {},
    { ...(signal ? { signal } : {}) },
  )
}
export function getFileURL(
  id: number,
  disposition: 'inline' | 'attachment',
  signal?: AbortSignal,
): Promise<FileURL> {
  return signal
    ? request.post<FileURL>(`/api/v1/storage/file/${id}/url`, { disposition }, { signal })
    : request.post<FileURL>(`/api/v1/storage/file/${id}/url`, { disposition })
}
export function deleteFile(id: number): Promise<{ accepted: true }> {
  return request.delete<{ accepted: true }>(`/api/v1/storage/file/${id}`)
}
