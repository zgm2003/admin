<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type * as taskApi from '@/api/message/notificationTask'
import type {
  NotificationLinkType,
  NotificationPriority,
  NotificationVariant,
} from '@/api/message/notification'
import NotificationEditor from '@/views/message/notificationTask/components/NotificationEditor/index.vue'
import NotificationTaskDetail from '@/views/message/notificationTask/components/NotificationTaskDetail/index.vue'
import type {
  NotificationTaskOptionKind,
  NotificationTaskOptionState,
  NotificationTaskOptionStates,
} from '@/views/message/notificationTask/useNotificationTaskOptions'

export type NotificationTaskFormModel = Omit<taskApi.NotificationTaskInput, 'platformId'> & {
  platformId: number | null
}

const props = defineProps<{
  modelValue: boolean
  title: string
  readonly: boolean
  loading: boolean
  errorMessage: string
  taskId: number | null
  detailTask: taskApi.NotificationTask | null
  form: NotificationTaskFormModel
  saving: boolean
  platformOptions: Array<{ value: number; label: string }>
  optionStates: NotificationTaskOptionStates
  targetState: NotificationTaskOptionState
  targetKind: NotificationTaskOptionKind
  audienceOptions: Array<{ value: taskApi.NotificationAudience; label: string }>
  variantOptions: Array<{ value: NotificationVariant; label: string }>
  priorityOptions: Array<{ value: NotificationPriority; label: string }>
  linkTypeOptions: Array<{ value: NotificationLinkType; label: string }>
  remotePlatformOptions: (keyword: string) => void
  remoteTargetOptions: (keyword: string) => void
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:form': [value: Partial<NotificationTaskFormModel>]
  save: []
  retry: [id: number, mode: 'edit' | 'detail']
  changePlatform: [value: number | null]
  changeAudience: [value: taskApi.NotificationAudience]
  changeLinkType: [value: NotificationLinkType]
  loadMore: [kind: NotificationTaskOptionKind]
}>()
const { t } = useI18n()

const close = (): void => emit('update:modelValue', false)
const setFormField = <K extends keyof NotificationTaskFormModel>(
  key: K,
  value: NotificationTaskFormModel[K],
): void => emit('update:form', { [key]: value })
</script>

<template>
  <AppDialog
    :model-value="props.modelValue"
    :title="props.title"
    width="760px"
    :height="props.readonly ? 'min(460px, 64vh)' : '70vh'"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="props.loading" class="notification-task-form__state">
      {{ t('notificationTask.loadingDetail') }}
    </div>
    <div v-else-if="props.errorMessage" class="notification-task-form__state">
      <span>{{ props.errorMessage }}</span>
      <el-button
        v-if="props.taskId !== null"
        link
        type="primary"
        @click="emit('retry', props.taskId, props.readonly ? 'detail' : 'edit')"
        >{{ t('notificationTask.retry') }}</el-button
      >
    </div>
    <NotificationTaskDetail
      v-else-if="props.readonly && props.detailTask !== null"
      :task="props.detailTask"
    />
    <el-form v-else class="notification-task-form" label-position="top">
      <div class="notification-task-form__grid">
        <el-form-item :label="t('notificationTask.platform')">
          <el-select-v2
            :model-value="props.form.platformId"
            data-testid="notification-task-platform"
            :options="props.platformOptions"
            :placeholder="t('notificationTask.platformPlaceholder')"
            filterable
            remote
            :loading="props.optionStates.platform.loading"
            :remote-method="props.remotePlatformOptions"
            @update:model-value="setFormField('platformId', $event)"
            @change="emit('changePlatform', $event)"
          />
        </el-form-item>
        <el-form-item :label="t('notificationTask.audienceLabel')">
          <el-select-v2
            :model-value="props.form.audienceType"
            data-testid="notification-task-audience"
            :options="props.audienceOptions"
            @update:model-value="setFormField('audienceType', $event)"
            @change="emit('changeAudience', $event)"
          />
        </el-form-item>
      </div>
      <el-form-item
        v-if="props.form.audienceType !== 'platform'"
        :label="t('notificationTask.targets')"
      >
        <el-select-v2
          :model-value="props.form.targetIds"
          data-testid="notification-task-targets"
          :options="props.targetState.items"
          multiple
          filterable
          remote
          :loading="props.targetState.loading"
          :remote-method="props.remoteTargetOptions"
          @update:model-value="setFormField('targetIds', $event)"
        />
        <el-button
          v-if="props.targetState.nextAfterId !== null"
          data-testid="notification-task-option-more"
          link
          @click="emit('loadMore', props.targetKind)"
          >{{ t('notificationTask.loadMoreOptions') }}</el-button
        >
        <span v-if="props.targetState.error" class="notification-task-form__error">{{
          props.targetState.error
        }}</span>
      </el-form-item>
      <el-button
        v-else-if="props.optionStates.platform.nextAfterId !== null"
        data-testid="notification-task-option-more"
        link
        @click="emit('loadMore', 'platform')"
        >{{ t('notificationTask.loadMoreOptions') }}</el-button
      >
      <el-form-item :label="t('notificationTask.title')">
        <el-input
          :model-value="props.form.title"
          data-testid="notification-task-title"
          maxlength="128"
          show-word-limit
          @update:model-value="setFormField('title', $event)"
        />
      </el-form-item>
      <div class="notification-task-form__grid notification-task-form__grid--three">
        <el-form-item :label="t('notificationTask.variantLabel')">
          <el-select-v2
            :model-value="props.form.variant"
            data-testid="notification-task-variant"
            :options="props.variantOptions"
            @update:model-value="setFormField('variant', $event)"
          />
        </el-form-item>
        <el-form-item :label="t('notificationTask.priorityLabel')">
          <el-select-v2
            :model-value="props.form.priority"
            data-testid="notification-task-priority"
            :options="props.priorityOptions"
            @update:model-value="setFormField('priority', $event)"
          />
        </el-form-item>
        <el-form-item :label="t('notificationTask.linkTypeLabel')">
          <el-select-v2
            :model-value="props.form.linkType"
            data-testid="notification-task-link-type"
            :options="props.linkTypeOptions"
            @update:model-value="setFormField('linkType', $event)"
            @change="emit('changeLinkType', $event)"
          />
        </el-form-item>
      </div>
      <el-form-item :label="t('notificationTask.content')">
        <NotificationEditor
          :model-value="props.form.contentHtml"
          @update:model-value="setFormField('contentHtml', $event)"
        />
      </el-form-item>
      <div class="notification-task-form__grid">
        <el-form-item :label="t('notificationTask.scheduledAt')">
          <el-date-picker
            :model-value="props.form.scheduledAt"
            class="notification-task-form__date-picker"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ss.SSSZ"
            clearable
            @update:model-value="setFormField('scheduledAt', $event)"
          />
        </el-form-item>
        <el-form-item v-if="props.form.linkType !== 'none'" :label="t('notificationTask.link')">
          <el-input
            :model-value="props.form.link"
            @update:model-value="setFormField('link', $event)"
          />
        </el-form-item>
      </div>
    </el-form>
    <template #footer>
      <el-button @click="close">{{ t('notificationTask.close') }}</el-button>
      <el-button
        v-if="!props.readonly"
        type="primary"
        :loading="props.saving"
        @click="emit('save')"
      >
        {{ t('notificationTask.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>
