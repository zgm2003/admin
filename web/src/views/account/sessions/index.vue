<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { getSessions, getSessionStats, revokeSession, revokeSessions } from '@/api/user/session'
import type { SessionItem, SessionListQuery, SessionStats, SessionStatus } from '@/api/user/session'
import { usePermissionStore } from '@/store/permission'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'

const { t } = useI18n()
const access = usePermissionStore()

const rows = ref<SessionItem[]>([])
const total = ref(0)
const stats = ref<SessionStats>({ activeTotal: 0, platforms: {} })
const query = ref<SessionListQuery>({ page: 1, pageSize: 20 })
const username = ref('')
const platform = ref('')
const status = ref<'' | SessionStatus>('')
const selectedIDs = ref<number[]>([])
const listLoading = ref(false)
const statsLoading = ref(false)
const mutating = ref(false)
const loadError = ref('')
const statsError = ref('')
const mutationError = ref('')

const tablePagination = computed<TablePaginationState | null>(() =>
  total.value > 0
    ? { currentPage: query.value.page, pageSize: query.value.pageSize, total: total.value }
    : null,
)
const tableColumns = computed<TableColumn<SessionItem>[]>(() => [
  { key: 'user', prop: 'id', label: t('session.column.user'), minWidth: 150 },
  { prop: 'platform', label: t('session.column.platform'), minWidth: 110 },
  { key: 'device', prop: 'id', label: t('session.column.device'), minWidth: 180 },
  { prop: 'clientIp', label: t('session.column.ip'), minWidth: 130 },
  { prop: 'userAgent', label: t('session.column.userAgent'), minWidth: 180, overflowTooltip: true },
  { key: 'createdAt', prop: 'id', label: t('session.column.createdAt'), minWidth: 180 },
  { key: 'expiresAt', prop: 'id', label: t('session.column.expiresAt'), minWidth: 180 },
  { key: 'status', prop: 'id', label: t('session.statusLabel'), width: 100 },
  { key: 'actions', prop: 'id', label: t('session.column.actions'), width: 120 },
])

const canRevoke = computed(() => access.hasPermission('auth:session:revoke'))
const platformStats = computed(() =>
  Object.entries(stats.value.platforms).sort(([left], [right]) => left.localeCompare(right)),
)
const searchModel = computed<SearchFormModel>({
  get: () => ({ username: username.value, platform: platform.value, status: status.value }),
  set: (value) => {
    username.value = typeof value.username === 'string' ? value.username : ''
    platform.value = typeof value.platform === 'string' ? value.platform : ''
    status.value =
      value.status === 'active' || value.status === 'expired' || value.status === 'revoked'
        ? value.status
        : ''
  },
})
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'username',
    type: 'input',
    label: t('session.username'),
    placeholder: t('session.username'),
    width: 220,
    testId: 'session-username',
  },
  {
    key: 'platform',
    type: 'input',
    label: t('session.platform'),
    placeholder: t('session.platform'),
    width: 180,
    testId: 'session-platform',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('session.statusLabel'),
    placeholder: t('session.statusLabel'),
    options: [
      { label: t('session.status.active'), value: 'active' },
      { label: t('session.status.expired'), value: 'expired' },
      { label: t('session.status.revoked'), value: 'revoked' },
    ],
    width: 160,
    testId: 'session-status',
  },
])

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== '' ? error.message : t(fallback)
}

async function loadSessions(): Promise<boolean> {
  if (listLoading.value) return false
  listLoading.value = true
  loadError.value = ''
  try {
    const result = await getSessions(query.value)
    rows.value = result.list
    total.value = result.total
    selectedIDs.value = []
    return true
  } catch (error: unknown) {
    loadError.value = errorMessage(error, 'session.loadFailed')
    return false
  } finally {
    listLoading.value = false
  }
}

async function loadStats(): Promise<boolean> {
  if (statsLoading.value) return false
  statsLoading.value = true
  statsError.value = ''
  try {
    stats.value = await getSessionStats()
    return true
  } catch (error: unknown) {
    statsError.value = errorMessage(error, 'session.loadFailed')
    return false
  } finally {
    statsLoading.value = false
  }
}

function buildQuery(page: number): SessionListQuery {
  const usernameValue = username.value.trim()
  const platformValue = platform.value.trim()
  return {
    page,
    pageSize: query.value.pageSize,
    ...(usernameValue === '' ? {} : { username: usernameValue }),
    ...(platformValue === '' ? {} : { platform: platformValue }),
    ...(status.value === '' ? {} : { status: status.value }),
  }
}

function search(): void {
  query.value = buildQuery(1)
  void loadSessions()
}

function reset(): void {
  username.value = ''
  platform.value = ''
  status.value = ''
  query.value = { page: 1, pageSize: query.value.pageSize }
  void loadSessions()
}

function changePage(page: number): void {
  query.value = { ...query.value, page }
  void loadSessions()
}

function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize }
  void loadSessions()
}

function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) {
    changePageSize(next.pageSize)
    return
  }
  changePage(next.currentPage)
}

function selectable(row: SessionItem): boolean {
  return canRevoke.value && row.status === 'active' && !row.isCurrent
}

function selectionChanged(selection: SessionItem[]): void {
  selectedIDs.value = selection
    .filter(selectable)
    .slice(0, 100)
    .map((row) => row.id)
}

async function reloadAuthoritativeData(): Promise<void> {
  await Promise.all([loadSessions(), loadStats()])
}

function canceled(error: unknown): boolean {
  return error === 'cancel' || error === 'close'
}

async function revokeOne(row: SessionItem): Promise<void> {
  if (!selectable(row) || mutating.value) return
  try {
    await ElMessageBox.confirm(t('session.revokeConfirm'), t('session.revoke'), { type: 'warning' })
    mutating.value = true
    mutationError.value = ''
    await revokeSession(row.id)
    await reloadAuthoritativeData()
    ElNotification.success({ title: t('session.revokeSuccess') })
  } catch (error: unknown) {
    if (!canceled(error)) mutationError.value = errorMessage(error, 'session.revokeFailed')
  } finally {
    mutating.value = false
  }
}

async function revokeSelected(): Promise<void> {
  if (!canRevoke.value || selectedIDs.value.length === 0 || mutating.value) return
  try {
    await ElMessageBox.confirm(t('session.revokeConfirm'), t('session.batchRevoke'), {
      type: 'warning',
    })
    mutating.value = true
    mutationError.value = ''
    await revokeSessions([...new Set(selectedIDs.value)].sort((left, right) => left - right))
    await reloadAuthoritativeData()
    ElNotification.success({ title: t('session.revokeSuccess') })
  } catch (error: unknown) {
    if (!canceled(error)) mutationError.value = errorMessage(error, 'session.revokeFailed')
  } finally {
    mutating.value = false
  }
}

function formatTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(
    new Date(value),
  )
}

function statusTagType(value: SessionStatus): 'success' | 'info' | 'danger' {
  if (value === 'active') return 'success'
  if (value === 'expired') return 'info'
  return 'danger'
}

onMounted(() => {
  void loadSessions()
  void loadStats()
})
</script>

<template>
  <section class="session-page management-page">
    <el-row :gutter="16" class="session-stats session-stats--compact" v-loading="statsLoading">
      <el-col :xs="24" :sm="8">
        <div class="session-stat-primary session-stat-primary--inline">
          <span>{{ t('session.activeTotal') }}</span>
          <strong>{{ stats.activeTotal }}</strong>
        </div>
      </el-col>
      <el-col :xs="24" :sm="16">
        <div class="session-platform-stats session-platform-stats--inline">
          <span class="session-platform-title">{{ t('session.platformDistribution') }}</span>
          <el-space wrap :size="8">
            <el-tag v-for="[code, count] in platformStats" :key="code" effect="plain"
              >{{ code }} · {{ count }}</el-tag
            >
          </el-space>
        </div>
      </el-col>
    </el-row>
    <el-alert v-if="statsError" :title="statsError" type="error" :closable="false" show-icon />

    <AppSearch
      v-model="searchModel"
      class="session-filters management-page__filters"
      :fields="searchFields"
      :query-label="t('session.search')"
      :reset-label="t('session.reset')"
      query-test-id="session-search"
      reset-test-id="session-reset"
      @query="search"
      @reset="reset"
    />

    <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
    <el-alert
      v-if="mutationError"
      :title="mutationError"
      type="error"
      :closable="false"
      show-icon
    />
    <div v-if="listLoading" data-testid="session-loading" :aria-label="t('session.loading')">
      <el-skeleton :rows="6" animated />
    </div>
    <el-empty v-else-if="rows.length === 0" :description="t('session.empty')" />
    <AppTable
      v-else
      data-testid="session-table"
      :columns="tableColumns"
      :data="rows"
      :selectable="canRevoke"
      :selection-selectable="selectable"
      :pagination="tablePagination"
      result-state="success"
      :aria-label="t('session.title')"
      :refresh-label="t('session.refresh')"
      @refresh="reloadAuthoritativeData"
      @selection-change="selectionChanged"
      @update:pagination="updateTablePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canRevoke"
          data-testid="session-batch-revoke"
          type="danger"
          plain
          :disabled="selectedIDs.length === 0"
          :loading="mutating"
          @click="revokeSelected"
        >
          {{ t('session.batchRevoke') }}
        </el-button>
      </template>
      <template #cell-user="{ row }: { row: SessionItem }">
        <strong>{{ row.username }}</strong>
        <small>#{{ row.userId }}</small>
      </template>
      <template #cell-device="{ row }: { row: SessionItem }"
        ><code v-if="row.id > 0">{{ row.deviceId }}</code></template
      >
      <template #cell-createdAt="{ row }: { row: SessionItem }">{{
        row.id > 0 ? formatTime(row.createdAt) : ''
      }}</template>
      <template #cell-expiresAt="{ row }: { row: SessionItem }">{{
        row.id > 0 ? formatTime(row.refreshExpiresAt) : ''
      }}</template>
      <template #cell-status="{ row }: { row: SessionItem }">
        <el-tag v-if="row.id > 0" :type="statusTagType(row.status)" effect="light">{{
          t(`session.status.${row.status}`)
        }}</el-tag>
      </template>
      <template #cell-actions="{ row }: { row: SessionItem }"
        ><template v-if="row.id > 0">
          <el-tag v-if="row.isCurrent" type="primary" effect="plain">{{
            t('session.current')
          }}</el-tag>
          <el-button
            v-else-if="canRevoke && row.status === 'active'"
            :data-testid="`session-revoke-${row.id}`"
            type="danger"
            text
            :loading="mutating"
            @click="revokeOne(row)"
          >
            {{ t('session.revoke') }}
          </el-button>
        </template></template
      >
      <template #empty><el-empty :description="t('session.empty')" /></template>
    </AppTable>
  </section>
</template>

<style scoped>
.session-page {
  min-width: 0;
}
.session-platform-stats {
  align-items: center;
}
.session-stats {
  align-items: center;
  min-height: 0;
  padding: 8px 14px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}
.session-stat-primary {
  display: flex;
  min-width: 140px;
  flex-direction: column;
  gap: 4px;
}
.session-stat-primary--inline {
  flex-direction: row;
  align-items: baseline;
  gap: 10px;
  min-height: 36px;
}
.session-stat-primary span,
.session-platform-title,
small {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.session-stat-primary strong {
  font-size: 24px;
  line-height: 1;
}
.session-platform-stats--inline {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 36px;
}
.session-filters .el-input {
  width: 220px;
}
.session-filters .el-select-v2 {
  width: 150px;
}
.el-table strong,
.el-table small {
  display: block;
}
.el-pagination {
  justify-content: flex-end;
}
@media (max-width: 900px) {
  .session-filters .el-input,
  .session-filters .el-select-v2 {
    width: 100%;
  }
}
</style>
