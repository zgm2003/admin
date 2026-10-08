import { requestUploadCredentials, type UploadCredentialItem } from '@/api/storage/upload'

export function sameMediaValue(left: string | string[], right: string | string[]): boolean {
  return Array.isArray(left) && Array.isArray(right)
    ? left.length === right.length && left.every((value, index) => value === right[index])
    : left === right
}

export class DirectUploadError extends Error {
  readonly reason: 'invalidType' | 'uploadFailed'
  constructor(reason: 'invalidType' | 'uploadFailed') {
    super(reason)
    this.reason = reason
  }
}

function contentType(file: File): string {
  if (file.name.toLowerCase().endsWith('.xlsx'))
    return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  // Windows may identify CSV as an Excel MIME type, or leave File.type empty.
  return file.name.toLowerCase().endsWith('.csv') ? 'text/csv' : file.type.toLowerCase()
}

function accepts(file: File, accept: string): boolean {
  const choices = accept
    .toLowerCase()
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  const xlsxOnly =
    choices.includes('.xlsx') &&
    choices.every(
      (choice) =>
        choice === '.xlsx' ||
        choice === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    )
  if (xlsxOnly && !file.name.toLowerCase().endsWith('.xlsx')) return false
  return (
    choices.length === 0 ||
    choices.some((choice) =>
      choice.startsWith('.')
        ? file.name.toLowerCase().endsWith(choice)
        : choice.endsWith('/*')
          ? contentType(file).startsWith(choice.slice(0, -1))
          : contentType(file) === choice,
    )
  )
}

export async function uploadMediaFiles(
  ruleCode: string,
  files: File[],
  accept: string,
  signal: AbortSignal,
  isCurrent: () => boolean,
): Promise<UploadCredentialItem[]> {
  const checkCurrent = () => {
    if (signal.aborted || !isCurrent()) throw new DOMException('Upload cancelled', 'AbortError')
  }
  checkCurrent()
  if (files.length === 0 || files.some((file) => !accepts(file, accept)))
    throw new DirectUploadError('invalidType')
  const credentials = await requestUploadCredentials(
    ruleCode,
    files.map((file) => ({
      fileName: file.name,
      contentType: contentType(file),
      fileSizeBytes: file.size,
    })),
  )
  checkCurrent()
  if (credentials.items.length !== files.length) throw new DirectUploadError('uploadFailed')
  for (const [index, item] of credentials.items.entries()) {
    checkCurrent()
    const file = files[index]
    if (!file) throw new DirectUploadError('uploadFailed')
    let response: Response
    try {
      response = await fetch(item.uploadUrl, {
        method: item.method,
        headers: item.headers,
        body: file,
        signal,
      })
    } catch (error: unknown) {
      if (signal.aborted) throw error
      throw new DirectUploadError('uploadFailed')
    }
    if (!response.ok) throw new DirectUploadError('uploadFailed')
  }
  checkCurrent()
  return credentials.items
}
