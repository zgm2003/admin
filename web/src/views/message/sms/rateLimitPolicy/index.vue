<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  updateSmsRateLimitPolicy,
  type SmsRateLimitPlatform,
  type SmsRateLimitPolicy,
} from '@/api/message/sms'
import { AppTable, type TableColumn } from '@/components/AppTable'

const props = defineProps<{
  platforms: SmsRateLimitPlatform[]
  loading: boolean
  canUpdate: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()

type PolicyRow = SmsRateLimitPolicy & {
  platformId: number
  platformCode: string
  platformName: string
  rowId: string
}
type Draft = { limit: number | null; windowSeconds: number | null }

const drafts = reactive<Record<string, Draft>>({})
const saving = reactive<Record<string, boolean>>({})

const rows = computed<PolicyRow[]>(() =>
  props.platforms.flatMap((platform) =>
    platform.policies.map((policy) => ({
      ...policy,
      platformId: platform.platformId,
      platformCode: platform.platformCode,
      platformName: platform.platformName,
      rowId: policyID(platform.platformId, policy.key),
    })),
  ),
)

watch(
  rows,
  (policies) => {
    for (const policy of policies) {
      if (drafts[policy.rowId] === undefined) {
        drafts[policy.rowId] = {
          limit: policy.limit,
          windowSeconds: policy.windowSeconds,
        }
      }
    }
  },
  { immediate: true },
)

const columns = computed<TableColumn<PolicyRow>[]>(() => [
  { key: 'platform', prop: 'platformName', label: t('sms.rateLimit.platform'), minWidth: 180 },
  { key: 'name', prop: 'key', label: t('sms.rateLimit.policyName'), minWidth: 220 },
  { key: 'mode', prop: 'mode', label: t('sms.rateLimit.mode'), width: 120 },
  { key: 'dimension', prop: 'dimension', label: t('sms.rateLimit.dimension'), minWidth: 170 },
  { key: 'limit', prop: 'key', label: t('sms.limit'), width: 190 },
  { key: 'window', prop: 'key', label: t('sms.windowSeconds'), width: 210 },
  { key: 'actions', prop: 'key', label: t('sms.actions'), width: 120, fixed: 'right' },
])

function policyID(platformID: number, key: SmsRateLimitPolicy['key']): string {
  return `${platformID}:${key}`
}

function draftOf(policy: PolicyRow): Draft {
  return drafts[policy.rowId] ?? { limit: null, windowSeconds: null }
}

function validDraft(policy: PolicyRow): boolean {
  const draft = draftOf(policy)
  if (draft.limit === null || draft.windowSeconds === null) return false
  return (
    draft.limit >= 1 &&
    draft.limit <= 100000 &&
    draft.windowSeconds >= 1 &&
    draft.windowSeconds <= 86400
  )
}

function dirty(policy: PolicyRow): boolean {
  const draft = draftOf(policy)
  return draft.limit !== policy.limit || draft.windowSeconds !== policy.windowSeconds
}

function updateLimit(policy: PolicyRow, value: number | undefined): void {
  draftOf(policy).limit = value ?? null
}

function updateWindow(policy: PolicyRow, value: number | undefined): void {
  draftOf(policy).windowSeconds = value ?? null
}

async function save(policy: PolicyRow): Promise<void> {
  const draft = draftOf(policy)
  const { limit, windowSeconds } = draft
  if (
    limit === null ||
    windowSeconds === null ||
    !validDraft(policy) ||
    !dirty(policy) ||
    saving[policy.rowId] === true
  ) {
    return
  }
  saving[policy.rowId] = true
  try {
    const result = await updateSmsRateLimitPolicy(policy.platformId, policy.key, {
      limit,
      windowSeconds,
    })
    const updated = result.policies.find((item) => item.key === policy.key)
    if (updated !== undefined) {
      drafts[policy.rowId] = {
        limit: updated.limit,
        windowSeconds: updated.windowSeconds,
      }
    }
    ElMessage.success(t('sms.rateLimit.saveSuccess'))
    emit('refresh')
  } catch {
    drafts[policy.rowId] = {
      limit: policy.limit,
      windowSeconds: policy.windowSeconds,
    }
  } finally {
    saving[policy.rowId] = false
  }
}
</script>

<template>
  <div class="sms-rate-limit">
    <el-alert
      class="sms-rate-limit__hint"
      :title="t('sms.rateLimit.explanation')"
      type="info"
      show-icon
      :closable="false"
    />
    <AppTable
      :columns="columns"
      :data="rows"
      row-key="rowId"
      :loading="loading"
      :aria-label="t('sms.tab.policies')"
      :refresh-label="t('sms.refresh')"
      @refresh="emit('refresh')"
    >
      <template #cell-platform="{ row }: { row: PolicyRow | undefined }">
        <template v-if="row?.platformId">{{ row.platformName }} ({{ row.platformCode }})</template>
      </template>
      <template #cell-name="{ row }: { row: PolicyRow | undefined }">
        <div v-if="row?.key" class="sms-rate-limit__name">
          <strong>{{ t(`sms.rateLimit.${row.key}`) }}</strong>
          <code>{{ row.key }}</code>
        </div>
      </template>
      <template #cell-mode="{ row }: { row: PolicyRow | undefined }">
        <template v-if="row?.key">{{ t('sms.rateLimit.modeBusiness') }}</template>
      </template>
      <template #cell-dimension="{ row }: { row: PolicyRow | undefined }">
        <template v-if="row?.key">{{ t('sms.rateLimit.dimensionPlatformPhone') }}</template>
      </template>
      <template #cell-limit="{ row }: { row: PolicyRow | undefined }">
        <el-input-number
          v-if="row?.key"
          :model-value="draftOf(row).limit"
          :min="1"
          :max="100000"
          :disabled="!canUpdate"
          :placeholder="t('sms.rateLimit.limitPlaceholder')"
          controls-position="right"
          :data-testid="`sms-rate-limit-${row.platformId}-${row.key}`"
          @update:model-value="updateLimit(row, $event)"
        />
      </template>
      <template #cell-window="{ row }: { row: PolicyRow | undefined }">
        <template v-if="row?.key">
          <el-input-number
            :model-value="draftOf(row).windowSeconds"
            :min="1"
            :max="86400"
            :disabled="!canUpdate"
            :placeholder="t('sms.rateLimit.windowPlaceholder')"
            controls-position="right"
            :data-testid="`sms-rate-window-${row.platformId}-${row.key}`"
            @update:model-value="updateWindow(row, $event)"
          />
          <span class="sms-rate-limit__unit">{{ t('sms.rateLimit.seconds') }}</span>
        </template>
      </template>
      <template #cell-actions="{ row }: { row: PolicyRow | undefined }">
        <el-button
          v-if="canUpdate && row?.key"
          text
          type="primary"
          :loading="saving[row.rowId] === true"
          :disabled="saving[row.rowId] === true"
          :data-testid="`sms-rate-save-${row.platformId}-${row.key}`"
          @click="save(row)"
        >
          {{ t('sms.save') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('sms.rateLimit.empty')" />
      </template>
    </AppTable>
  </div>
</template>

<style scoped>
.sms-rate-limit {
  min-width: 0;
}

.sms-rate-limit__hint {
  margin-bottom: 12px;
}

.sms-rate-limit__name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sms-rate-limit__name code,
.sms-rate-limit__unit {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.sms-rate-limit__unit {
  margin-left: 6px;
}
</style>
