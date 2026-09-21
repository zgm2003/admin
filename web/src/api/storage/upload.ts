import { request } from '@/utils/request'
import { expectArray, expectExactKeys, expectRecord, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'

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
  return parseCredentials(
    await request({
      method: 'POST',
      url: '/api/v1/storage/upload-credential',
      data: { ruleCode, files },
    }),
  )
}

export async function requestObjectURL(objectKey: string): Promise<ObjectURLResult> {
  const value = expectExactKeys(
    await request({
      method: 'POST',
      url: '/api/v1/storage/object-url',
      data: { objectKey },
    }),
    ['url', 'expiresAt'] as const,
    'object url',
  )
  const url = expectString(value.url, 'object url.url')
  if (url.trim() === '') throw new ProtocolError('object url.url must not be empty')
  if (value.expiresAt === null) return { url, expiresAt: null }
  const expiresAt = expectString(value.expiresAt, 'object url.expiresAt')
  if (!expiresAt.endsWith('Z') || Number.isNaN(Date.parse(expiresAt))) {
    throw new ProtocolError('object url.expiresAt must be an ISO UTC timestamp or null')
  }
  return { url, expiresAt }
}

function parseCredentials(value: unknown): UploadCredentialResponse {
  const item = expectRecord(value, 'upload credentials')
  return {
    items: expectArray(item.items, 'upload credentials.items').map((entry, index) => {
      const credential = expectRecord(entry, `upload credentials.items[${index}]`)
      if (credential.method !== 'PUT')
        throw new ProtocolError('upload credential method is invalid')
      const headersRecord = expectRecord(
        credential.headers,
        `upload credentials.items[${index}].headers`,
      )
      const headers: Record<string, string> = {}
      for (const [key, header] of Object.entries(headersRecord))
        headers[key] = expectString(header, `upload credentials.headers.${key}`)
      return {
        uploadUrl: expectString(credential.uploadUrl, 'upload credential.uploadUrl'),
        objectKey: expectString(credential.objectKey, 'upload credential.objectKey'),
        method: 'PUT',
        headers,
        expiresAt: expectString(credential.expiresAt, 'upload credential.expiresAt'),
        publicUrl:
          credential.publicUrl === undefined
            ? undefined
            : expectString(credential.publicUrl, 'upload credential.publicUrl'),
      }
    }),
  }
}
