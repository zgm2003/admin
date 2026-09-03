import type { TableColumn } from '@/components/AppTable'
import type { SearchField } from '@/components/AppSearch'
import { YesNo } from '@/enums/yes-no'
import type { CosConfig } from '@/api/storage/cosconfig'
import type { ConfigSummary, PlatformOption, UploadRule } from '@/api/storage/uploadrule'

type Translate = (key: string) => string

export function createConfigSearchFields(t: Translate): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('storage.keyword'),
      placeholder: t('storage.keyword'),
      width: 260,
      testId: 'storage-config-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('storage.status'),
      options: [
        { label: t('storage.allStatus'), value: '' },
        { label: t('storage.enabled'), value: YesNo.Yes },
        { label: t('storage.disabled'), value: YesNo.No },
      ],
      width: 170,
    },
  ]
}

export function createRuleSearchFields(
  t: Translate,
  platforms: readonly PlatformOption[],
  configs: readonly ConfigSummary[],
): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('storage.keyword'),
      placeholder: t('storage.keyword'),
      width: 220,
      testId: 'storage-rule-keyword',
    },
    {
      key: 'platform',
      type: 'select-v2',
      label: t('storage.platform'),
      options: [
        { label: t('storage.allPlatforms'), value: '' },
        ...platforms.map((item) => ({ label: item.name, value: item.id })),
      ],
      width: 180,
    },
    {
      key: 'config',
      type: 'select-v2',
      label: t('storage.config'),
      options: [
        { label: t('storage.allConfigs'), value: '' },
        ...configs.map((item) => ({ label: item.name, value: item.id })),
      ],
      width: 180,
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('storage.status'),
      options: [
        { label: t('storage.allStatus'), value: '' },
        { label: t('storage.enabled'), value: YesNo.Yes },
        { label: t('storage.disabled'), value: YesNo.No },
      ],
      width: 170,
    },
  ]
}

export function createConfigColumns(t: Translate): TableColumn<CosConfig>[] {
  return [
    { prop: 'name', label: t('storage.name'), minWidth: 160 },
    { prop: 'bucket', label: t('storage.bucket'), minWidth: 190 },
    { prop: 'region', label: t('storage.region'), width: 150 },
    { key: 'credentials', prop: 'id', label: t('storage.credentials'), width: 130 },
    { key: 'status', prop: 'id', label: t('storage.status'), width: 110 },
    { key: 'actions', prop: 'id', label: t('storage.actions'), width: 310 },
  ]
}

export function createRuleColumns(t: Translate): TableColumn<UploadRule>[] {
  return [
    { prop: 'name', label: t('storage.name'), minWidth: 150 },
    { prop: 'platformName', label: t('storage.platform'), width: 150 },
    { prop: 'cosConfigName', label: t('storage.config'), width: 170 },
    { key: 'codes', prop: 'codes', label: t('storage.code'), minWidth: 220 },
    { key: 'status', prop: 'id', label: t('storage.status'), width: 110 },
    { key: 'actions', prop: 'id', label: t('storage.actions'), width: 250 },
  ]
}
