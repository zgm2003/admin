import type { AuthPlatformListItem } from '@/api/auth/platform'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

type Translate = (key: string, params?: Record<string, unknown>) => string

export function authPlatformSearchFields(t: Translate): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('authPlatform.keyword'),
      placeholder: t('authPlatform.keyword'),
      width: 260,
      testId: 'auth-platform-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('authPlatform.status.all'),
      placeholder: t('authPlatform.status.all'),
      options: [
        { label: t('authPlatform.status.all'), value: '' },
        { label: t('authPlatform.status.enabled'), value: YesNo.Yes },
        { label: t('authPlatform.status.disabled'), value: YesNo.No },
      ],
      width: 160,
      testId: 'auth-platform-status-filter',
    },
  ]
}

export function authPlatformTableColumns(t: Translate): TableColumn<AuthPlatformListItem>[] {
  return [
    { key: 'platform', prop: 'id', label: t('authPlatform.column.platform'), minWidth: 160 },
    { key: 'tokenTTL', prop: 'id', label: t('authPlatform.column.tokenTTL'), minWidth: 175 },
    { key: 'cacheTTL', prop: 'id', label: t('authPlatform.column.cacheTTL'), minWidth: 175 },
    { key: 'security', prop: 'id', label: t('authPlatform.column.security'), minWidth: 165 },
    { key: 'sessions', prop: 'id', label: t('authPlatform.column.sessions'), width: 110 },
    {
      key: 'registration',
      prop: 'id',
      label: t('authPlatform.column.registration'),
      width: 105,
    },
    { key: 'status', prop: 'id', label: t('authPlatform.column.status'), width: 90 },
    { prop: 'updatedAt', label: t('authPlatform.column.updatedAt'), width: 140 },
    {
      key: 'actions',
      prop: 'id',
      label: t('authPlatform.column.actions'),
      width: 190,
      fixed: 'right',
    },
  ]
}

export function authPlatformSessionLabel(value: number, t: Translate): string {
  if (value === 0) return t('authPlatform.unlimited')
  if (value === 1) return t('authPlatform.singleSession')
  return t('authPlatform.maxSessions', { count: value })
}

export function authPlatformTTLLabel(value: number, t: Translate): string {
  if (value % 86_400 === 0) return t('authPlatform.readableDays', { count: value / 86_400 })
  if (value % 3_600 === 0) return t('authPlatform.readableHours', { count: value / 3_600 })
  if (value % 60 === 0) return t('authPlatform.readableMinutes', { count: value / 60 })
  return t('authPlatform.seconds', { count: value })
}

export function formatAuthPlatformDate(value: string, locale: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(parsed)
}

export function formatAuthPlatformTime(value: string, locale: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return new Intl.DateTimeFormat(locale, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(parsed)
}
