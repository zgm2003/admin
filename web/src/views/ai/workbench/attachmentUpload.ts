import { completeFileUpload, type CompletedFile } from '@/api/storage/file'
import { requestUploadCredentials } from '@/api/storage/upload'

const contentTypes: Record<string, string> = {
  csv: 'text/csv',
  txt: 'text/plain',
  log: 'text/plain',
  md: 'text/markdown',
  json: 'application/json',
  xml: 'application/xml',
  yaml: 'application/x-yaml',
  yml: 'application/x-yaml',
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  ppt: 'application/vnd.ms-powerpoint',
  pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  rar: 'application/vnd.rar',
  '7z': 'application/x-7z-compressed',
}

function contentType(file: File): string {
  const extension = file.name.split('.').at(-1)?.toLowerCase() ?? ''
  return contentTypes[extension] ?? (file.type || 'application/octet-stream')
}

export async function uploadAttachmentFiles(files: File[]): Promise<CompletedFile[]> {
  const credentials = await requestUploadCredentials(
    'file',
    files.map((file) => ({
      fileName: file.name,
      contentType: contentType(file),
      fileSizeBytes: file.size,
    })),
  )
  if (credentials.items.length !== files.length) throw new Error('upload credential mismatch')

  const completed: CompletedFile[] = []
  for (const [index, item] of credentials.items.entries()) {
    const file = files[index]
    if (file === undefined) throw new Error('upload file missing')
    const response = await fetch(item.uploadUrl, {
      method: item.method,
      headers: item.headers,
      body: file,
    })
    if (!response.ok) throw new Error('upload request failed')
    completed.push(await completeFileUpload(item.fileId))
  }
  return completed
}
