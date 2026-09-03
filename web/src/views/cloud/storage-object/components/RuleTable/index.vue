<script setup lang="ts">
import { CirclePlus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { UploadRule } from '@/api/storage/uploadrule'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

const props = defineProps<{
  columns: TableColumn<UploadRule>[]
  data: UploadRule[]
  fields: SearchField[]
  loading: boolean
  pagination: TablePaginationState
  can: (code: string) => boolean
  canAdd: boolean
  canCreate: boolean
  canUpdate: boolean
  missingPrerequisite: boolean
}>()
const model = defineModel<SearchFormModel>({ required: true })
const emit = defineEmits<{
  delete: [row: UploadRule]
  open: [row?: UploadRule]
  pagination: [value: TablePaginationState]
  query: []
  refresh: []
  reset: []
  status: [row: UploadRule]
}>()
const { t } = useI18n()
</script>

<template>
  <AppSearch
    v-model="model"
    :fields="props.fields"
    :query-label="t('storage.search')"
    :reset-label="t('storage.reset')"
    query-test-id="storage-rule-search"
    reset-test-id="storage-rule-reset"
    @query="emit('query')"
    @reset="emit('reset')"
  />
  <el-alert
    v-if="props.canCreate && props.missingPrerequisite"
    :title="t('storage.rulePrerequisite')"
    type="warning"
    show-icon
  />
  <AppTable
    :columns="props.columns"
    :data="props.data"
    :loading="props.loading"
    :pagination="props.pagination"
    result-state="success"
    :aria-label="t('storage.rulesTab')"
    :refresh-label="t('storage.refresh')"
    @refresh="emit('refresh')"
    @update:pagination="emit('pagination', $event)"
  >
    <template #toolbar-left>
      <el-button
        v-if="props.canCreate"
        type="primary"
        :icon="CirclePlus"
        data-testid="storage-add-rule"
        :disabled="!props.canAdd"
        @click="emit('open')"
      >
        {{ t('storage.addRule') }}
      </el-button>
    </template>
    <template #cell-codes="{ row }">
      <el-space wrap :size="4">
        <el-tag v-for="code in row.codes" :key="code" size="small">{{ code }}</el-tag>
      </el-space>
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
          v-if="props.can('storage:upload-rule:status')"
          text
          type="warning"
          @click="emit('status', row)"
        >
          {{ row.isEnabled === YesNo.Yes ? t('storage.disable') : t('storage.enable') }}
        </el-button>
        <el-button
          v-if="props.can('storage:upload-rule:delete')"
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
