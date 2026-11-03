<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppPage from '@/components/AppPage/index.vue'
import AppTable from '@/components/AppTable/index.vue'
import AppDialog from '@/components/AppDialog/index.vue'
import RunRecordTable from '@/views/ai/components/RunRecordTable/index.vue'
import type { TableColumn } from '@/components/AppTable'
import type { BackendOption } from '@/types/option'
import { usePermissionStore } from '@/store/permission'
import {
  getRun,
  listRuns,
  getRunOptions,
  getRunSummary,
  listRunRecords,
  listRunMedia,
  previewRunMedia,
  type RunItem,
  type RunDetail,
  type RunFilter,
  type RunOptions,
  type RunRecordKind,
  type RunRequest,
  type RunToolCall,
  type RunCharge,
  type RunChargeItem,
  type RunMedia,
  type RunSummary,
} from '@/api/ai/run'
const { t, locale } = useI18n(),
  access = usePermissionStore()
const canList = computed(() => access.hasPermission('ai:run:list')),
  canDetail = computed(() => access.hasPermission('ai:run:detail'))
const rows = ref<RunItem[]>([]),
  options = ref<RunOptions | null>(null),
  summary = ref<RunSummary | null>(null)
const filters = ref<RunFilter>({
    limit: 20,
    startAt: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
    endAt: new Date().toISOString(),
  }),
  loading = ref(false),
  failed = ref(false),
  next = ref(''),
  hasMore = ref(false)
const detail = ref<RunDetail | null>(null),
  visible = ref(false),
  detailLoading = ref(false),
  detailFailed = ref(false)
const section = ref<RunRecordKind>('request'),
  requests = ref<RunRequest[]>([]),
  tools = ref<RunToolCall[]>([]),
  charges = ref<RunCharge[]>([]),
  chargeItems = ref<RunChargeItem[]>([]),
  recordLoading = ref(false),
  media = ref<RunMedia[]>([]),
  recordNext = ref(''),
  recordMore = ref(false),
  mediaNext = ref(''),
  mediaMore = ref(false)
const preview = ref(''),
  previewMime = ref(''),
  previewVisible = ref(false)
let sequence = 0,
  detailSequence = 0
function statusLabel(items: BackendOption<number>[] | undefined, value: number): string {
  return items?.find((item) => item.value === value)?.label ?? String(value)
}
const columns = computed<TableColumn<RunItem>[]>(() => [
  { prop: 'id', label: t('aiRun.id'), minWidth: 100 },
  { prop: 'operation', label: t('aiRun.operation'), minWidth: 150 },
  {
    prop: 'executionStatus',
    label: t('aiRun.executionStatus'),
    formatter: (_r, value) => statusLabel(options.value?.executionStatuses, Number(value)),
  },
  {
    prop: 'usageStatus',
    label: t('aiRun.usageStatus'),
    formatter: (_r, value) => statusLabel(options.value?.usageStatuses, Number(value)),
  },
  {
    prop: 'billingStatus',
    label: t('aiRun.billingStatus'),
    formatter: (_r, value) => statusLabel(options.value?.billingStatuses, Number(value)),
  },
  {
    prop: 'transferStatus',
    label: t('aiRun.transferStatus'),
    formatter: (_r, value) => statusLabel(options.value?.transferStatuses, Number(value)),
  },
  { prop: 'estimatedUnits', label: t('aiRun.estimatedUnits') },
  { prop: 'acceptedAt', label: t('aiRun.acceptedAt'), minWidth: 180 },
  { key: 'actions', label: t('common.actions'), width: 100 },
])
async function load(append = false) {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  failed.value = false
  try {
    const result = await listRuns(
      { ...filters.value, beforeId: append ? next.value : undefined },
      true,
    )
    if (current !== sequence) return
    rows.value = append ? [...rows.value, ...result.list] : result.list
    next.value = result.nextBeforeId
    hasMore.value = result.hasMore
    if (!append) summary.value = null
    if (
      !append &&
      filters.value.startAt &&
      filters.value.endAt &&
      options.value &&
      new Date(filters.value.endAt).getTime() - new Date(filters.value.startAt).getTime() <=
        options.value.constraints.maxSummaryHours * 3600000
    ) {
      const value = await getRunSummary({
        ...filters.value,
        beforeId: undefined,
        limit: undefined,
      })
      if (current === sequence) summary.value = value
    }
  } catch {
    if (current === sequence) failed.value = true
  } finally {
    if (current === sequence) loading.value = false
  }
}
async function open(row: RunItem) {
  if (!canDetail.value) return
  const current = ++detailSequence
  visible.value = true
  detail.value = null
  clearRecords()
  media.value = []
  detailLoading.value = true
  detailFailed.value = false
  try {
    const value = await getRun(row.id, true)
    if (current !== detailSequence) return
    detail.value = value
    await Promise.all([loadRecords(), loadMedia()])
  } catch {
    detailFailed.value = true
  } finally {
    detailLoading.value = false
  }
}
async function loadRecords(append = false) {
  if (!detail.value) return
  const id = detail.value.id,
    kind = section.value
  const query = { limit: 20, beforeId: append ? recordNext.value : undefined }
  const current = () => id === detail.value?.id && kind === section.value
  recordLoading.value = true
  try {
    switch (kind) {
      case 'request': {
        const result = await listRunRecords(id, kind, query)
        if (!current()) return
        requests.value = append ? [...requests.value, ...result.list] : result.list
        recordNext.value = result.nextBeforeId
        recordMore.value = result.hasMore
        break
      }
      case 'tool-call': {
        const result = await listRunRecords(id, kind, query)
        if (!current()) return
        tools.value = append ? [...tools.value, ...result.list] : result.list
        recordNext.value = result.nextBeforeId
        recordMore.value = result.hasMore
        break
      }
      case 'usage-charge': {
        const result = await listRunRecords(id, kind, query)
        if (!current()) return
        charges.value = append ? [...charges.value, ...result.list] : result.list
        recordNext.value = result.nextBeforeId
        recordMore.value = result.hasMore
        break
      }
      case 'usage-charge-item': {
        const result = await listRunRecords(id, kind, query)
        if (!current()) return
        chargeItems.value = append ? [...chargeItems.value, ...result.list] : result.list
        recordNext.value = result.nextBeforeId
        recordMore.value = result.hasMore
        break
      }
    }
  } catch {
    if (current()) detailFailed.value = true
  } finally {
    if (current()) recordLoading.value = false
  }
}
function clearRecords() {
  requests.value = []
  tools.value = []
  charges.value = []
  chargeItems.value = []
  recordNext.value = ''
  recordMore.value = false
}
async function loadMedia(append = false) {
  if (!detail.value) return
  const id = detail.value.id
  try {
    const result = await listRunMedia(
      detail.value.id,
      { limit: 20, beforeId: append ? mediaNext.value : undefined },
      true,
    )
    if (detail.value?.id !== id) return
    media.value = append ? [...media.value, ...result.list] : result.list
    mediaNext.value = result.nextBeforeId
    mediaMore.value = result.hasMore
  } catch {
    detailFailed.value = true
  }
}
async function showMedia(item: RunMedia, download = false) {
  if (!detail.value || !canDetail.value) return
  try {
    const value = await previewRunMedia(
      detail.value.id,
      item.id,
      download ? 'attachment' : 'inline',
      true,
    )
    if (download) {
      window.open(value.url, '_blank', 'noopener,noreferrer')
      return
    }
    preview.value = value.url
    previewMime.value = item.mimeType ?? ''
    previewVisible.value = true
  } catch {
    detailFailed.value = true
  }
}
async function loadOptions() {
  try {
    options.value = await getRunOptions(true)
  } catch {
    failed.value = true
  }
}
watch(locale, () => void loadOptions())
watch(section, () => {
  clearRecords()
  void loadRecords()
})
watch(visible, (v) => {
  if (!v) ++detailSequence
})
onMounted(() => {
  void loadOptions()
  void load()
})
</script>
<template>
  <AppPage
    ><div class="ai-run">
      <div class="ai-run__filters">
        <el-input
          v-for="field in [
            'userId',
            'platformId',
            'agentId',
            'providerId',
            'providerModelId',
          ] as const"
          :key="field"
          v-model="filters[field]"
          :placeholder="t(`aiRun.${field}`)"
          clearable
        />
        <el-date-picker
          v-model="filters.startAt"
          type="datetime"
          value-format="YYYY-MM-DDTHH:mm:ssZ"
          :placeholder="t('aiRun.startAt')"
        /><el-date-picker
          v-model="filters.endAt"
          type="datetime"
          value-format="YYYY-MM-DDTHH:mm:ssZ"
          :placeholder="t('aiRun.endAt')"
        />
        <el-select-v2
          v-model="filters.operation"
          :options="options?.operations ?? []"
          clearable
          :placeholder="t('aiRun.operation')"
        />
        <el-select-v2
          v-model="filters.executionStatus"
          :options="options?.executionStatuses ?? []"
          clearable
          :placeholder="t('aiRun.executionStatus')"
        />
        <el-select-v2
          v-model="filters.usageStatus"
          :options="options?.usageStatuses ?? []"
          clearable
          :placeholder="t('aiRun.usageStatus')"
        /><el-select-v2
          v-model="filters.billingStatus"
          :options="options?.billingStatuses ?? []"
          clearable
          :placeholder="t('aiRun.billingStatus')"
        /><el-select-v2
          v-model="filters.transferStatus"
          :options="options?.transferStatuses ?? []"
          clearable
          :placeholder="t('aiRun.transferStatus')"
        />
        <el-button v-if="canList" :loading="loading" @click="load()">{{
          t('aiRun.refresh')
        }}</el-button>
      </div>
      <el-alert v-if="!canList" :title="t('aiChat.noPermission')" :closable="false" />
      <p v-if="summary">
        {{
          t('aiRun.summary', {
            count: summary.runCount,
            estimated: summary.estimatedUnits,
            settled: summary.settledUnits,
          })
        }}
      </p>
      <AppTable
        :data="rows"
        :columns="columns"
        row-key="id"
        :loading="loading"
        :result-state="failed ? 'error' : rows.length ? 'success' : 'empty'"
        @refresh="load()"
        ><template #cell-actions="{ row }"
          ><el-button v-if="canDetail" data-testid="ai-run-detail" link @click="open(row)">{{
            t('common.detail')
          }}</el-button></template
        ></AppTable
      >
      <el-button v-if="hasMore" :loading="loading" @click="load(true)">{{
        t('aiChat.loadMore')
      }}</el-button>
      <AppDialog v-model="visible" :title="t('aiRun.title')" width="1000px"
        ><el-skeleton v-if="detailLoading" animated /><el-alert
          v-if="detailFailed"
          type="error"
          :closable="false"
          :title="t('aiChat.loadFailed')"
        /><template v-if="detail"
          ><p>{{ detail.id }} · {{ detail.errorCode }} {{ detail.errorMessage }}</p>
          <h4>{{ t('aiRun.input') }}</h4>
          <pre>{{ JSON.stringify(detail.input, null, 2) }}</pre>
          <h4>{{ t('aiRun.result') }}</h4>
          <pre>{{ JSON.stringify(detail.result, null, 2) }}</pre>
          <h4>{{ t('aiRun.configuration') }}</h4>
          <pre>{{ JSON.stringify(detail.configuration, null, 2) }}</pre>
          <el-radio-group v-model="section"
            ><el-radio-button value="request">{{ t('aiRun.requests') }}</el-radio-button
            ><el-radio-button value="tool-call">{{ t('aiRun.tools') }}</el-radio-button
            ><el-radio-button value="usage-charge-item">{{
              t('aiRun.chargeItems')
            }}</el-radio-button
            ><el-radio-button value="usage-charge">{{
              t('aiRun.charges')
            }}</el-radio-button></el-radio-group
          >
          <RunRecordTable
            :kind="section"
            :requests="requests"
            :tools="tools"
            :charges="charges"
            :charge-items="chargeItems"
            :options="options"
            :loading="recordLoading"
            @refresh="loadRecords()"
          />
          <el-button v-if="recordMore" :loading="recordLoading" @click="loadRecords(true)">{{
            t('aiChat.loadMore')
          }}</el-button>
          <h4>{{ t('aiRun.media') }}</h4>
          <article v-for="item in media" :key="item.id">
            <p>
              {{ item.id }} · {{ item.mimeType }} ·
              {{ statusLabel(options?.mediaTransferStatuses, item.transferStatus) }}
              {{ item.errorMessage }}
            </p>
            <el-button :disabled="!item.canPreview" @click="showMedia(item)">{{
              t('aiChat.preview')
            }}</el-button
            ><el-button :disabled="!item.canPreview" @click="showMedia(item, true)">{{
              t('aiChat.download')
            }}</el-button>
          </article>
          <el-button v-if="mediaMore" @click="loadMedia(true)">{{
            t('aiChat.loadMore')
          }}</el-button></template
        ></AppDialog
      >
      <AppDialog v-model="previewVisible" :title="t('aiChat.preview')"
        ><img
          v-if="previewMime.startsWith('image/')"
          :src="preview"
          class="ai-run__preview"
        /><audio v-else-if="previewMime.startsWith('audio/')" controls :src="preview" /><video
          v-else-if="previewMime.startsWith('video/')"
          controls
          :src="preview"
          class="ai-run__preview"
        /><a v-else :href="preview" target="_blank" rel="noopener noreferrer">{{
          t('aiChat.download')
        }}</a></AppDialog
      >
    </div></AppPage
  >
</template>
<style scoped lang="scss">
.ai-run {
  padding: 16px;
}
.ai-run__filters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}
.ai-run__preview {
  max-width: 100%;
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: var(--el-fill-color-light);
  padding: 12px;
}
article {
  border-bottom: 1px solid var(--el-border-color);
  padding: 8px;
}
</style>
