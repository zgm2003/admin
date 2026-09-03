<script setup lang="ts">
import { CirclePlus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { CosConfig } from '@/api/storage/cosconfig'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

const props = defineProps<{
  columns: TableColumn<CosConfig>[]
  data: CosConfig[]
  fields: SearchField[]
  loading: boolean
  pagination: TablePaginationState
  can: (code: string) => boolean
  canCreate: boolean
  canUpdate: boolean
}>()
const model = defineModel<SearchFormModel>({ required: true })
const emit = defineEmits<{
  delete: [row: CosConfig]
  open: [row?: CosConfig]
  pagination: [value: TablePaginationState]
  query: []
  refresh: []
  reset: []
  status: [row: CosConfig]
  test: [row: CosConfig]
}>()
const { t } = useI18n()
</script>

<template>
  <AppSearch
    v-model="model"
    :fields="props.fields"
    :query-label="t('storage.search')"
    :reset-label="t('storage.reset')"
    query-test-id="storage-config-search"
    reset-test-id="storage-config-reset"
    @query="emit('query')"
    @reset="emit('reset')"
  />
  <AppTable
    :columns="props.columns"
    :data="props.data"
    :loading="props.loading"
    :pagination="props.pagination"
    :aria-label="t('storage.configTab')"
    :refresh-label="t('storage.refresh')"
    @refresh="emit('refresh')"
    @update:pagination="emit('pagination', $event)"
  >
    <template #toolbar-left>
      <el-button
        v-if="props.canCreate"
        type="primary"
        :icon="CirclePlus"
        data-testid="storage-add-config"
        @click="emit('open')"
      >
        {{ t('storage.addConfig') }}
      </el-button>
    </template>
    <template #cell-credentials="{ row }">
      <el-tag size="small" :type="row.hasCredentials ? 'success' : 'warning'">
        {{ row.hasCredentials ? t('storage.configured') : t('storage.missing') }}
      </el-tag>
    </template>
    <template #cell-status="{ row }">
      <el-tag size="small" :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">
        {{ row.isEnabled === YesNo.Yes ? t('storage.enabled') : t('storage.disabled') }}
      </el-tag>
    </template>
    <template #cell-actions="{ row }">
      <el-space wrap :size="4">
        <el-button v-if="props.canUpdate" text type="primary" @click="emit('open', row)">
          {{ t('storage.edit') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cos-config:test')"
          text
          type="primary"
          @click="emit('test', row)"
        >
          {{ t('storage.test') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cos-config:status')"
          text
          type="warning"
          @click="emit('status', row)"
        >
          {{ row.isEnabled === YesNo.Yes ? t('storage.disable') : t('storage.enable') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cos-config:delete')"
          text
          type="danger"
          @click="emit('delete', row)"
        >
          {{ t('storage.delete') }}
        </el-button>
      </el-space>
    </template>
  </AppTable>
</template>
