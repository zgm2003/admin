import type { TableColumn } from '@/components/AppTable'
import type { SearchField } from '@/components/AppSearch'
import { YesNo } from '@/enums/yesNo'
import type { CosConfig } from '@/api/storage/cosConfig'
import type { ConfigSummary, PlatformOption, UploadRule } from '@/api/storage/uploadRule'

type Translate = (key: string) => string
export interface StorageConfigSearchModel {
  keyword: string
  status: '' | YesNo
}
export interface StorageRuleSearchModel {
  keyword: string
  platform: '' | number
  config: '' | number
  status: '' | YesNo
}

export function createConfigSearchFields(t: Translate): SearchField<StorageConfigSearchModel>[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      resetValue: '',
      label: t('storage.keyword'),
      placeholder: t('storage.keyword'),
      width: 260,
      testId: 'storage-config-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      resetValue: '',
      label: t('storage.status'),
      placeholder: t('storage.allStatus'),
      options: [
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
): SearchField<StorageRuleSearchModel>[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      resetValue: '',
      label: t('storage.keyword'),
      placeholder: t('storage.keyword'),
      width: 220,
      testId: 'storage-rule-keyword',
    },
    {
      key: 'platform',
      type: 'select-v2',
      resetValue: '',
      label: t('storage.platform'),
      options: platforms.map((item) => ({ label: item.name, value: item.id })),
      placeholder: t('storage.allPlatforms'),
      width: 180,
    },
    {
      key: 'config',
      type: 'select-v2',
      resetValue: '',
      label: t('storage.config'),
      options: configs.map((item) => ({ label: item.name, value: item.id })),
      placeholder: t('storage.allConfigs'),
      width: 180,
    },
    {
      key: 'status',
      type: 'select-v2',
      resetValue: '',
      label: t('storage.status'),
      placeholder: t('storage.allStatus'),
      options: [
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
