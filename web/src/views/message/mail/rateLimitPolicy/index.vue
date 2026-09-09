<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { updateMailRateLimitPolicy, type MailRateLimitPolicy } from '@/api/message/mail'
import { AppTable, type TableColumn } from '@/components/AppTable'

const props = defineProps<{
  policies: MailRateLimitPolicy[]
  loading: boolean
  canUpdate: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()

type Draft = { limit: number | null; windowSeconds: number | null }
const drafts = reactive<Record<string, Draft>>({})
const saving = reactive<Record<string, boolean>>({})

watch(
  () => props.policies,
  (policies) => {
    for (const policy of policies) {
      if (drafts[policy.key] === undefined) {
        drafts[policy.key] = { limit: policy.limit, windowSeconds: policy.windowSeconds }
      }
    }
  },
  { immediate: true },
)

const columns = computed<TableColumn<MailRateLimitPolicy>[]>(() => [
  { key: 'name', prop: 'key', label: t('mail.rateLimit.policy'), minWidth: 220 },
  { key: 'mode', prop: 'mode', label: t('mail.rateLimit.mode'), width: 130 },
  { key: 'dimension', prop: 'dimension', label: t('mail.rateLimit.dimension'), minWidth: 180 },
  { key: 'limit', prop: 'key', label: t('mail.rateLimit.limit'), width: 190 },
  { key: 'window', prop: 'key', label: t('mail.rateLimit.window'), width: 190 },
  { key: 'actions', prop: 'key', label: t('mail.actions'), width: 140, fixed: 'right' },
])

function draftOf(key: string): Draft {
  return drafts[key] ?? { limit: null, windowSeconds: null }
}

function validDraft(policy: MailRateLimitPolicy): boolean {
  const draft = draftOf(policy.key)
  if (draft.limit === null || draft.windowSeconds === null) return false
  return (
    draft.limit >= 1 &&
    draft.limit <= 100000 &&
    draft.windowSeconds >= 1 &&
    draft.windowSeconds <= 86400
  )
}

function dirty(policy: MailRateLimitPolicy): boolean {
  const draft = draftOf(policy.key)
  return draft.limit !== policy.limit || draft.windowSeconds !== policy.windowSeconds
}

async function save(policy: MailRateLimitPolicy): Promise<void> {
  const draft = draftOf(policy.key)
  if (draft.limit === null || draft.windowSeconds === null || saving[policy.key]) return
  saving[policy.key] = true
  try {
    const result = await updateMailRateLimitPolicy(policy.key, {
      limit: draft.limit,
      windowSeconds: draft.windowSeconds,
    })
    drafts[policy.key] = { limit: result.policy.limit, windowSeconds: result.policy.windowSeconds }
    ElMessage.success(t('mail.rateLimit.saveSuccess'))
    emit('refresh')
  } catch {
    drafts[policy.key] = { limit: policy.limit, windowSeconds: policy.windowSeconds }
  } finally {
    saving[policy.key] = false
  }
}
</script>

<template>
  <div class="table-tab">
    <el-alert
      class="rate-limit-hint"
      :title="t('mail.rateLimit.explanation')"
      type="info"
      show-icon
      :closable="false"
    />
    <AppTable
      :columns="columns"
      :data="policies"
      row-key="key"
      :loading="loading"
      :aria-label="t('mail.rateLimitsTab')"
      :refresh-label="t('mail.refresh')"
      @refresh="emit('refresh')"
    >
      <template #cell-name="{ row }: { row: MailRateLimitPolicy | undefined }">
        <template v-if="row?.key">
          <div class="rate-limit-name">
            <strong>{{ t(`mail.rateLimit.policy.${row.key}`) }}</strong>
            <code>{{ row.key }}</code>
          </div>
        </template>
      </template>
      <template #cell-mode="{ row }: { row: MailRateLimitPolicy | undefined }">
        <template v-if="row?.key">
          {{
            t(
              row.mode === 'business'
                ? 'mail.rateLimit.modeBusiness'
                : 'mail.rateLimit.modeAdminTest',
            )
          }}
        </template>
      </template>
      <template #cell-dimension="{ row }: { row: MailRateLimitPolicy | undefined }">
        <template v-if="row?.key">{{ t(`mail.rateLimit.dimension.${row.dimension}`) }}</template>
      </template>
      <template #cell-limit="{ row }: { row: MailRateLimitPolicy | undefined }">
        <template v-if="row?.key">
          <el-input-number
            v-model="draftOf(row.key).limit"
            :min="1"
            :max="100000"
            :disabled="!canUpdate"
            :placeholder="t('mail.rateLimit.limitPlaceholder')"
            controls-position="right"
            data-testid="rate-limit-input"
          />
        </template>
      </template>
      <template #cell-window="{ row }: { row: MailRateLimitPolicy | undefined }">
        <template v-if="row?.key">
          <el-input-number
            v-model="draftOf(row.key).windowSeconds"
            :min="1"
            :max="86400"
            :disabled="!canUpdate"
            :placeholder="t('mail.rateLimit.windowPlaceholder')"
            controls-position="right"
            data-testid="rate-limit-window-input"
          />
          <span class="rate-limit-unit">{{ t('mail.rateLimit.seconds') }}</span>
        </template>
      </template>
      <template #cell-actions="{ row }: { row: MailRateLimitPolicy | undefined }">
        <el-button
          v-if="canUpdate && row?.key"
          text
          type="primary"
          :loading="saving[row.key] === true"
          :disabled="!validDraft(row) || !dirty(row)"
          :data-testid="`rate-limit-save-${row.key}`"
          @click="save(row)"
        >
          {{ t('mail.save') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.rateLimit.empty')" />
      </template>
    </AppTable>
  </div>
</template>

<style scoped>
.table-tab {
  min-width: 0;
}

.rate-limit-hint {
  margin-bottom: 12px;
}

.rate-limit-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rate-limit-name code {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.rate-limit-unit {
  margin-left: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
