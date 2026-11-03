import { ApiError, ProtocolError } from '@/types/http'

/** Keep transport/API details in request notifications; inline errors are local UI fallbacks. */
export function displayError(error: unknown, fallback: string): string {
  if (error instanceof ApiError || error instanceof ProtocolError) return fallback
  return error instanceof Error && error.message !== '' ? error.message : fallback
}
