<script setup lang="ts">
import { computed } from 'vue'
import { CirclePlus, Delete, Edit, Switch } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { Dictionary, DictionaryItem } from '@/api/system/dictionary'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { getDictionaryUsage } from '../../dictionaryUsage'

defineOptions({ name: 'DictionaryDetailDialog' })

const props = defineProps<{
  dictionary: Dictionary | null
  items: DictionaryItem[]
  canCreateItems: boolean
  canUpdate: boolean
  canStatus: boolean
  canDelete: boolean
}>()

const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{
  refresh: []
  'create-item': []
  'edit-item': [item: DictionaryItem]
  'toggle-item': [item: DictionaryItem]
  'remove-item': [item: DictionaryItem]
}>()
const { t } = useI18n()
const usage = computed(() => getDictionaryUsage(props.dictionary?.code ?? ''))
const canCreateItems = computed(
  () => props.canCreateItems && usage.value.valuePolicy === 'extensible',
)
const canMutateItemValues = computed(() => usage.value.valuePolicy === 'extensible')

const itemColumns = computed<TableColumn<DictionaryItem>[]>(() => [
  { prop: 'value', label: t('dictionary.value'), minWidth: 140 },
  { prop: 'labelZh', label: t('dictionary.labelZh'), minWidth: 140 },
  { prop: 'labelEn', label: t('dictionary.labelEn'), minWidth: 140 },
  { prop: 'sort', label: t('dictionary.sort'), width: 80 },
  { key: 'itemStatus', prop: 'id', label: t('dictionary.status'), width: 90 },
  { key: 'itemActions', prop: 'id', label: t('dictionary.actions'), width: 230 },
])
</script>

<template>
  <AppDialog v-model="visible" :title="dictionary?.code ?? t('dictionary.items')" width="900px">
    <el-alert
      :title="t(usage.consumerLabelKey)"
      :description="`${t(usage.impactLabelKey)} ${t(usage.valueHintKey)}`"
      type="info"
      :closable="false"
      show-icon
      class="dictionary-detail__usage"
    />
    <AppTable
      :data="items"
      :columns="itemColumns"
      row-key="id"
      :aria-label="t('dictionary.items')"
      :refresh-label="t('appTable.refresh')"
      @refresh="emit('refresh')"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreateItems"
          data-testid="dictionary-item-create"
          type="primary"
          :icon="CirclePlus"
          @click="emit('create-item')"
          >{{ t('dictionary.createItem') }}</el-button
        >
      </template>
      <template #cell-itemStatus="{ row }: { row: DictionaryItem }">
        <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">
          {{ row.isEnabled === YesNo.Yes ? t('dictionary.enabled') : t('dictionary.disabled') }}
        </el-tag>
      </template>
      <template #cell-itemActions="{ row }: { row: DictionaryItem }">
        <el-button
          v-if="canUpdate"
          data-testid="dictionary-item-update"
          text
          :icon="Edit"
          @click="emit('edit-item', row)"
          >{{ t('dictionary.edit') }}</el-button
        >
        <el-button
          v-if="canStatus && canMutateItemValues"
          data-testid="dictionary-item-status"
          text
          :icon="Switch"
          @click="emit('toggle-item', row)"
          >{{
            row.isEnabled === YesNo.Yes ? t('dictionary.disable') : t('dictionary.enable')
          }}</el-button
        >
        <el-button
          v-if="canDelete && canMutateItemValues && row.isBuiltin === YesNo.No"
          data-testid="dictionary-item-delete"
          text
          type="danger"
          :icon="Delete"
          @click="emit('remove-item', row)"
          >{{ t('dictionary.delete') }}</el-button
        >
      </template>
    </AppTable>
  </AppDialog>
</template>
