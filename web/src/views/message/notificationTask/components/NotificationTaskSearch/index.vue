<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { notificationTaskAudienceMetadata } from '@/api/message/notificationTask'

interface NotificationTaskSearchModel {
  keyword: string
  platformId: string
  audienceType: '' | 'user' | 'role' | 'platform'
  timeRange: [] | [string, string]
}

const model = defineModel<SearchFormModel<NotificationTaskSearchModel>>({ required: true })
const emit = defineEmits<{ query: []; reset: [] }>()
const { t } = useI18n()

const audienceOptions = computed<
  Array<{ value: NotificationTaskSearchModel['audienceType']; label: string }>
>(() => [
  { value: '', label: t('notificationTask.audienceAll') },
  ...notificationTaskAudienceMetadata.map((item) => ({
    value: item.value,
    label: t(item.i18nKey),
  })),
])
const fields = computed<SearchField<NotificationTaskSearchModel>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    resetValue: '',
    label: t('notificationTask.keyword'),
    placeholder: t('notificationTask.keywordPlaceholder'),
    clearable: true,
    testId: 'notification-task-keyword',
  },
  {
    key: 'platformId',
    type: 'input',
    resetValue: '',
    label: t('notificationTask.platformId'),
    placeholder: t('notificationTask.platformIdPlaceholder'),
    clearable: true,
    width: 150,
    testId: 'notification-task-platform-id',
  },
  {
    key: 'audienceType',
    type: 'select-v2',
    resetValue: '',
    label: t('notificationTask.audienceLabel'),
    placeholder: t('notificationTask.audiencePlaceholder'),
    options: audienceOptions.value,
    clearable: true,
    width: 170,
  },
  {
    key: 'timeRange',
    type: 'date-range',
    resetValue: [],
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
