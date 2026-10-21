<!-- 组件：对象存储配置表格 -->
<script setup lang="ts">
import { CirclePlus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { CosConfig } from '@/api/storage/cosConfig'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import type { StorageConfigSearchModel } from '@/views/storage/object/storageView'

const props = defineProps<{
  columns: TableColumn<CosConfig>[]
  data: CosConfig[]
  fields: SearchField<StorageConfigSearchModel>[]
  loading: boolean
  pagination: TablePaginationState
  can: (code: string) => boolean
  canCreate: boolean
  canUpdate: boolean
}>()
const model = defineModel<SearchFormModel<StorageConfigSearchModel>>({ required: true })
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
        {{
          row.isEnabled === YesNo.Yes ? t('common.enable') : t('common.disable')
        }}
      </el-tag>
    </template>
    <template #cell-actions="{ row }">
      <el-space wrap :size="4">
        <el-button v-if="props.canUpdate" text type="primary" @click="emit('open', row)">
          {{ t('common.edit') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cosConfig:test')"
          text
          type="primary"
          @click="emit('test', row)"
        >
          {{ t('storage.test') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cosConfig:status')"
          text
          :type="row.isEnabled === YesNo.Yes ? 'warning' : 'success'"
          @click="emit('status', row)"
        >
          {{ row.isEnabled === YesNo.Yes ? t('common.disable') : t('common.enable') }}
        </el-button>
        <el-button
          v-if="props.can('storage:cosConfig:delete')"
          text
          type="danger"
          @click="emit('delete', row)"
        >
          {{ t('common.delete') }}
        </el-button>
      </el-space>
    </template>
  </AppTable>
</template>
