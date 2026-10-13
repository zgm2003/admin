import type { RequestClient } from '@/utils/request'
import { createUploadApi } from '@/api/storage/upload'

export async function uploadAvatar(client: RequestClient, file: File, signal?: AbortSignal, storageFetch: typeof fetch = fetch) {
  if (file.size === 0 || !file.type.startsWith('image/')) throw new Error('请选择非空的图片文件')
  const api = createUploadApi(client)
  const { items } = await api.credentials(file, signal)
  const credential = items[0]
  if (items.length !== 1 || !credential) throw new Error('服务器未返回唯一上传凭证')
  const response = await storageFetch(credential.uploadUrl, { method: credential.method, headers: credential.headers, body: file, credentials: 'omit', signal })
  if (!response.ok) throw new Error(`图片上传失败（${response.status}），请重新选择文件重试`)
  return api.complete(credential.fileId, signal)
}
