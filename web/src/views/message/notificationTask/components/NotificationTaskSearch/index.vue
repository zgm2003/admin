<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { SearchField, SearchFormModel } from '@/components/AppSearch'

const model = defineModel<SearchFormModel>({ required: true })
const emit = defineEmits<{ query: []; reset: [] }>()
const { t } = useI18n()

const statusOptions = computed(() => [
  { value: '', label: t('notificationTask.statusAll') },
  ...(
    ['draft', 'scheduled', 'queued', 'processing', 'completed', 'failed', 'canceled'] as const
  ).map((value) => ({ value, label: t(`notificationTask.status.${value}`) })),
])
const audienceOptions = computed(() => [
  { value: '', label: t('notificationTask.audienceAll') },
  ...(['platform', 'user', 'role'] as const).map((value) => ({
    value,
    label: t(`notificationTask.audience.${value}`),
  })),
])
const fields = computed<SearchField[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('notificationTask.keyword'),
    placeholder: t('notificationTask.keywordPlaceholder'),
    clearable: true,
    testId: 'notification-task-keyword',
  },
  {
    key: 'platformId',
    type: 'input',
    label: t('notificationTask.platformId'),
    placeholder: t('notificationTask.platformIdPlaceholder'),
    clearable: true,
    width: 150,
    testId: 'notification-task-platform-id',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('notificationTask.statusLabel'),
    placeholder: t('notificationTask.statusPlaceholder'),
    options: statusOptions.value,
    clearable: true,
    width: 160,
  },
  {
    key: 'audienceType',
    type: 'select-v2',
    label: t('notificationTask.audienceLabel'),
    placeholder: t('notificationTask.audiencePlaceholder'),
    options: audienceOptions.value,
    clearable: true,
    width: 170,
  },
  {
    key: 'timeRange',
    type: 'date-range',
    label: t('notificationTask.timeRange'),
    startPlaceholder: t('notificationTask.timeRangeStartPlaceholder'),
    endPlaceholder: t('notificationTask.timeRangeEndPlaceholder'),
    valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    rangeSeparator: '-',
    width: 360,
  },
])
</script>

<template>
  <AppSearch
    v-model="model"
    class="management-page__filters"
    :fields="fields"
    :query-label="t('search.query')"
    :reset-label="t('search.reset')"
    query-test-id="notification-task-search"
    reset-test-id="notification-task-reset"
    @query="emit('query')"
    @reset="emit('reset')"
  />
</template>
