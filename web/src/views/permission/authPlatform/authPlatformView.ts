import type { AuthPlatformListItem } from '@/api/permission/authPlatform'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'

type Translate = (key: string, params?: Record<string, unknown>) => string

export function authPlatformSearchFields(t: Translate): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('permission.authPlatform.keyword'),
      placeholder: t('permission.authPlatform.keyword'),
      width: 260,
      testId: 'auth-platform-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('permission.authPlatform.status.all'),
      placeholder: t('permission.authPlatform.status.all'),
      options: [
        { label: t('permission.authPlatform.status.all'), value: '' },
        { label: t('permission.authPlatform.status.enabled'), value: YesNo.Yes },
        { label: t('permission.authPlatform.status.disabled'), value: YesNo.No },
      ],
      width: 160,
      testId: 'auth-platform-status-filter',
    },
  ]
}

export function authPlatformTableColumns(t: Translate): TableColumn<AuthPlatformListItem>[] {
  return [
    {
      key: 'platform',
      prop: 'id',
      label: t('permission.authPlatform.column.platform'),
      minWidth: 160,
    },
    {
      key: 'tokenTTL',
      prop: 'id',
      label: t('permission.authPlatform.column.tokenTTL'),
      minWidth: 175,
    },
    {
      key: 'cacheTTL',
      prop: 'id',
      label: t('permission.authPlatform.column.cacheTTL'),
      minWidth: 175,
    },
    {
      key: 'security',
      prop: 'id',
      label: t('permission.authPlatform.column.security'),
      minWidth: 165,
    },
    {
      key: 'sessions',
      prop: 'id',
      label: t('permission.authPlatform.column.sessions'),
      width: 110,
    },
    {
      key: 'registration',
      prop: 'id',
      label: t('permission.authPlatform.column.registration'),
      width: 105,
    },
    { key: 'status', prop: 'id', label: t('permission.authPlatform.column.status'), width: 90 },
    { prop: 'updatedAt', label: t('permission.authPlatform.column.updatedAt'), width: 140 },
    {
      key: 'actions',
      prop: 'id',
      label: t('permission.authPlatform.column.actions'),
      width: 190,
      fixed: 'right',
    },
  ]
}

export function authPlatformSessionLabel(value: number, t: Translate): string {
  if (value === 0) return t('permission.authPlatform.unlimited')
  if (value === 1) return t('permission.authPlatform.singleSession')
  return t('permission.authPlatform.maxSessions', { count: value })
}

export function authPlatformTTLLabel(value: number, t: Translate): string {
  if (value % 86_400 === 0)
    return t('permission.authPlatform.readableDays', { count: value / 86_400 })
  if (value % 3_600 === 0)
    return t('permission.authPlatform.readableHours', { count: value / 3_600 })
  if (value % 60 === 0) return t('permission.authPlatform.readableMinutes', { count: value / 60 })
  return t('permission.authPlatform.seconds', { count: value })
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
