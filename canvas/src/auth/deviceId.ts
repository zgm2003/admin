const storageKey = 'canvas.deviceId'

export function getDeviceId(): string {
  const stored = window.localStorage.getItem(storageKey)
  if (stored !== null && isUUID(stored)) return stored
  const value = crypto.randomUUID()
  window.localStorage.setItem(storageKey, value)
  return value
}

function isUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value)
}
