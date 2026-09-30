// Matches the server's storage/uploadRule v2 key protocol; URLs are never object keys.
export function isStorageObjectKey(value: string): boolean {
  if (value.length > 1024 || value.includes('..')) return false
  const match =
    /^([a-z0-9][a-z0-9._-]*(?:\/[a-z0-9][a-z0-9._-]*)*)\/\.admin-storage\/v2\/p([1-9]\d*)\/r([1-9]\d*)\/c([1-9]\d*)\/v([1-9]\d*)\/(\d{4})\/(\d{2})\/(\d{2})\/[0-9a-f]{32}\.[a-z0-9]{1,16}$/u.exec(
      value,
    )
  if (!match || !match[1] || match[1].length > 64) return false
  if (match[1].split('/').includes('.admin-storage')) return false
  for (const id of match.slice(2, 6)) {
    if (BigInt(id) > 9223372036854775807n) return false
  }
  const year = Number(match[6])
  const month = Number(match[7])
  const day = Number(match[8])
  const date = new Date(0)
  date.setUTCFullYear(year, month - 1, day)
  return (
    date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
  )
}
