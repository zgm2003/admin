<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { EmailChangeLogItem } from '@/api/user/email'
import type { PhoneChangeLogItem } from '@/api/user/phone'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { formatTime } from '@/utils/datetime'

defineOptions({ name: 'EmailChangeLogDialog' })
const props = defineProps<{
  modelValue: boolean
  username: string
  rows: EmailChangeLogItem[]
  loading: boolean
  error: string
  pagination: TablePaginationState
  phoneRows: PhoneChangeLogItem[]
  phoneLoading: boolean
  phoneError: string
  phonePagination: TablePaginationState
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:pagination': [value: TablePaginationState]
  'update:phone-pagination': [value: TablePaginationState]
  refresh: []
  'refresh:phone': []
}>()
const { t } = useI18n()
const activeTab = ref<'email' | 'phone'>('email')
const columns = computed<TableColumn<EmailChangeLogItem>[]>(() => [
  { key: 'action', prop: 'action', label: t('user.emailChangeAction'), width: 120 },
  { prop: 'oldEmail', label: t('user.emailChangeOldEmail'), minWidth: 220 },
  { prop: 'newEmail', label: t('user.emailChangeNewEmail'), minWidth: 220 },
  { prop: 'platform', label: t('user.emailChangePlatform'), width: 120 },
  { prop: 'createdAt', label: t('user.emailChangeCreatedAt'), minWidth: 190 },
])
const phoneColumns = computed<TableColumn<PhoneChangeLogItem>[]>(() => [
  { key: 'action', prop: 'action', label: t('user.emailChangeAction'), width: 120 },
  { prop: 'oldPhone', label: t('user.phoneChangeOldPhone'), minWidth: 190 },
  { prop: 'newPhone', label: t('user.phoneChangeNewPhone'), minWidth: 190 },
  { prop: 'platform', label: t('user.emailChangePlatform'), width: 120 },
  { prop: 'createdAt', label: t('user.emailChangeCreatedAt'), minWidth: 190 },
])
function actionLabel(action: 1 | 2): string {
  return t(action === 1 ? 'user.emailChangeActionChange' : 'user.emailChangeActionBind')
}
</script>

<template>
  <AppDialog
    :model-value="props.modelValue"
    :title="`${t('user.emailChangeTitle')} · ${props.username}`"
    width="1100px"
    height="560px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-tabs v-model="activeTab" stretch>
      <el-tab-pane :label="t('user.emailChangeTitle')" name="email">
        <el-alert v-if="props.error" :title="props.error" type="error" show-icon />
        <AppTable
          :columns="columns"
          :data="props.rows"
          :loading="props.loading"
          :pagination="props.pagination"
          :aria-label="t('user.emailChangeTitle')"
          @refresh="emit('refresh')"
          @update:pagination="emit('update:pagination', $event)"
        >
          <template #cell-action="{ row }: { row: EmailChangeLogItem }"
            ><el-tag :type="row.action === 1 ? 'warning' : 'success'">{{
              actionLabel(row.action)
            }}</el-tag></template
          >
          <template #cell-oldEmail="{ row }: { row: EmailChangeLogItem }">{{
            row.oldEmail ?? '-'
          }}</template>
          <template #cell-createdAt="{ row }: { row: EmailChangeLogItem }">{{
            formatTime(row.createdAt)
          }}</template>
          <template #empty><el-empty :description="t('user.emailChangeEmpty')" /></template>
        </AppTable>
      </el-tab-pane>
      <el-tab-pane :label="t('user.phoneChangeTitle')" name="phone">
        <el-alert v-if="props.phoneError" :title="props.phoneError" type="error" show-icon />
        <AppTable
          :columns="phoneColumns"
          :data="props.phoneRows"
          :loading="props.phoneLoading"
          :pagination="props.phonePagination"
          :aria-label="t('user.phoneChangeTitle')"
          @refresh="emit('refresh:phone')"
          @update:pagination="emit('update:phone-pagination', $event)"
        >
          <template #cell-action="{ row }: { row: PhoneChangeLogItem }"
            ><el-tag :type="row.action === 1 ? 'warning' : 'success'">{{
              actionLabel(row.action)
            }}</el-tag></template
          >
          <template #cell-oldPhone="{ row }: { row: PhoneChangeLogItem }">{{
            row.oldPhone ?? '-'
          }}</template>
          <template #cell-createdAt="{ row }: { row: PhoneChangeLogItem }">{{
            formatTime(row.createdAt)
          }}</template>
          <template #empty><el-empty :description="t('user.phoneChangeEmpty')" /></template>
        </AppTable>
      </el-tab-pane>
    </el-tabs>
  </AppDialog>
</template>
