import type { TaskOption } from '@/api/system/scheduler'
import type { BackendOption } from '@/types/option'

// This sentinel is an editor state, not a business cron expression.
export const CUSTOM_CRON_VALUE = '__custom__'

export function getCronPresetValue(
  expression: string,
  presets: readonly BackendOption<string>[],
): string {
  const value = expression.trim()
  return presets.some((preset) => preset.value === value) ? value : CUSTOM_CRON_VALUE
}

export function resolveOptionLabel<T extends string | number | boolean>(
  value: T,
  options: readonly BackendOption<T>[],
): string {
  return options.find((option) => option.value === value)?.label ?? String(value)
}

export function resolveTaskDisplayName(type: string, options: readonly TaskOption[]): string {
  return options.find((option) => option.type === type)?.displayName ?? type
}
