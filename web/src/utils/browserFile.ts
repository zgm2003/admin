export function decodeBase64Bytes(value: string): ArrayBuffer {
  if (value.length % 4 !== 0 || !/^[A-Za-z0-9+/]*={0,2}$/.test(value)) {
    throw new Error('Invalid Base64 content')
  }
  let binary: string
  try {
    binary = atob(value)
  } catch {
    throw new Error('Invalid Base64 content')
  }
  if (btoa(binary) !== value) throw new Error('Invalid Base64 content')
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index)
  return bytes.buffer
}
