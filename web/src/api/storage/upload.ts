import { request } from '@/utils/request'
import { expectArray, expectRecord, expectString } from '@/api/protocol'
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

export async function requestUploadCredentials(
  ruleCode: string,
  files: UploadFileInput[],
): Promise<UploadCredentialResponse> {
  return parseCredentials(
    await request<unknown>({
      method: 'POST',
      url: '/api/v1/storage/upload-credentials',
      data: { ruleCode, files },
    }),
  )
}

export async function requestObjectURL(
  ruleCode: string,
  objectKey: string,
): Promise<{ url: string }> {
  const value = expectRecord(
    await request<unknown>({
      method: 'POST',
      url: '/api/v1/storage/object-url',
      data: { ruleCode, objectKey },
    }),
    'object url',
  )
  return { url: expectString(value.url, 'object url.url') }
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
