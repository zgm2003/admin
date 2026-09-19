import { ProtocolError } from '@/types/http'

const leaseMilliseconds = 6_000
export const leaderHeartbeatMilliseconds = 2_000
export const followerCheckMilliseconds = 1_000

export function realtimeChannelName(platformCode: string, userId: number): string {
  return `admin:realtime:v1:${platformCode}:${userId}`
}
export function leaderLeaseKey(platformCode: string, userId: number): string {
  return `admin:realtime:leader:v1:${platformCode}:${userId}`
}

interface LeaderOptions {
  platformCode: string
  userId: number
  storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>
  now?: () => number
  randomId?: () => string
}

export function createLeaderElector(options: LeaderOptions) {
  const now = options.now ?? Date.now
  const owner = (options.randomId ?? (() => crypto.randomUUID()))()
  const key = leaderLeaseKey(options.platformCode, options.userId)
  const read = (): { schemaVersion: 1; owner: string; expiresAt: number } | null => {
    const raw = options.storage.getItem(key)
    if (raw === null) return null
    try {
      const value: unknown = JSON.parse(raw)
      if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error()
      const r = value as Record<string, unknown>
      if (
        Object.keys(r).length !== 3 ||
        r.schemaVersion !== 1 ||
        typeof r.owner !== 'string' ||
        r.owner === '' ||
        typeof r.expiresAt !== 'number' ||
        !Number.isInteger(r.expiresAt)
      )
        throw new Error()
      return { schemaVersion: 1, owner: r.owner, expiresAt: r.expiresAt }
    } catch {
      options.storage.removeItem(key)
      return null
    }
  }
  const owns = () => {
    const lease = read()
    return lease !== null && lease.owner === owner && lease.expiresAt > now()
  }
  return {
    owner,
    tryAcquire(): boolean {
      const current = read()
      if (current !== null && current.expiresAt > now() && current.owner !== owner) return false
      options.storage.setItem(
        key,
        JSON.stringify({ schemaVersion: 1, owner, expiresAt: now() + leaseMilliseconds }),
      )
      return owns()
    },
    heartbeat(): boolean {
      if (!owns()) return false
      options.storage.setItem(
        key,
        JSON.stringify({ schemaVersion: 1, owner, expiresAt: now() + leaseMilliseconds }),
      )
      return owns()
    },
    isLeader: owns,
    release(): void {
      const current = read()
      if (current?.owner === owner) options.storage.removeItem(key)
    },
  }
}

export function validateLeaderIdentity(platformCode: string, userId: number): void {
  if (platformCode.trim() === '' || !Number.isInteger(userId) || userId <= 0)
    throw new ProtocolError('invalid realtime leader identity')
}
