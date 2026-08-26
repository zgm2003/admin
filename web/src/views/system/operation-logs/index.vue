<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { getOperationLogs } from '../../../api/operation-log'
import type { OperationLogItem, OperationLogListQuery } from '../../../api/operation-log.contract'
import { YesNo } from '../../../enums/yes-no'
import { AppTable } from '../../../components/AppTable'
import type { TableColumn, TablePaginationState } from '../../../components/AppTable'
import { Search } from '../../../components/Search'
import type { SearchField, SearchFormModel } from '../../../components/Search'

const { t } = useI18n()

const rows = ref<OperationLogItem[]>([])
const total = ref(0)
const query = ref<OperationLogListQuery>({ page: 1, pageSize: 20 })
const userID = ref('')
const action = ref('')
const route = ref('')
const success = ref<'' | YesNo>('')
const timeRange = ref<[] | [string, string]>([])
const loading = ref(false)
const loadError = ref('')
const searchModel = computed<SearchFormModel>({
  get: () => ({ userID: userID.value, action: action.value, route: route.value, success: success.value, timeRange: timeRange.value }),
  set: (value) => {
    userID.value = typeof value.userID === 'string' ? value.userID : ''
    action.value = typeof value.action === 'string' ? value.action : ''
    route.value = typeof value.route === 'string' ? value.route : ''
    success.value = value.success === YesNo.Yes || value.success === YesNo.No ? value.success : ''
    timeRange.value = Array.isArray(value.timeRange) && value.timeRange.length === 2 && value.timeRange.every((item) => typeof item === 'string') ? [value.timeRange[0] as string, value.timeRange[1] as string] : []
  },
})
const searchFields = computed<SearchField[]>(() => [
  { key: 'userID', type: 'input', label: t('operationLog.userId'), placeholder: t('operationLog.userId'), width: 180, testId: 'operation-log-user-id' },
  { key: 'action', type: 'input', label: t('operationLog.action'), placeholder: t('operationLog.action'), width: 190, testId: 'operation-log-action' },
  { key: 'route', type: 'input', label: t('operationLog.route'), placeholder: t('operationLog.route'), width: 220, testId: 'operation-log-route' },
  { key: 'success', type: 'select-v2', label: t('operationLog.successLabel'), options: [{ label: t('operationLog.all'), value: '' }, { label: t('operationLog.success.yes'), value: YesNo.Yes }, { label: t('operationLog.success.no'), value: YesNo.No }], width: 130, testId: 'operation-log-success' },
  { key: 'timeRange', type: 'date-range', label: t('operationLog.timeRange'), placeholder: t('operationLog.timeRange'), valueFormat: 'YYYY-MM-DDTHH:mm:ssZ', rangeSeparator: '-', width: 360, testId: 'operation-log-time' },
])

const tablePagination = computed<TablePaginationState>(() => ({ currentPage: query.value.page, pageSize: query.value.pageSize, total: total.value }))
const tableColumns = computed<TableColumn<OperationLogItem>[]>(() => [
  { key: 'expand', prop: 'id', label: '', width: 48, expand: true },
  { key: 'method', prop: 'id', label: t('operationLog.column.method'), width: 90 },
  { prop: 'action', label: t('operationLog.column.action'), minWidth: 160, overflowTooltip: true },
  { prop: 'route', label: t('operationLog.column.route'), minWidth: 220, overflowTooltip: true },
  { key: 'user', prop: 'id', label: t('operationLog.column.user'), width: 100 },
  { prop: 'clientIp', label: t('operationLog.column.ip'), minWidth: 130 },
  { key: 'status', prop: 'id', label: t('operationLog.column.status'), width: 100 },
  { key: 'latency', prop: 'id', label: t('operationLog.column.latency'), width: 100 },
  { key: 'createdAt', prop: 'id', label: t('operationLog.column.createdAt'), minWidth: 180 },
])

function errorMessage(error: unknown): string {
	return error instanceof Error && error.message !== '' ? error.message : t('operationLog.loadFailed')
}

async function loadLogs(): Promise<boolean> {
	if (loading.value) return false
	loading.value = true
	loadError.value = ''
	try {
		const result = await getOperationLogs(query.value)
		rows.value = result.list
		total.value = result.total
		return true
	} catch (error: unknown) {
		loadError.value = errorMessage(error)
		return false
	} finally {
		loading.value = false
	}
}

function buildQuery(page: number): OperationLogListQuery | null {
	const userIDValue = userID.value.trim()
	const actionValue = action.value.trim()
	const routeValue = route.value.trim()
	let parsedUserID: number | undefined
	if (userIDValue !== '') {
		parsedUserID = Number(userIDValue)
		if (!Number.isInteger(parsedUserID) || parsedUserID <= 0) {
			loadError.value = t('operationLog.userIdInvalid')
			return null
		}
	}
	return {
		page,
		pageSize: query.value.pageSize,
		...(parsedUserID === undefined ? {} : { userId: parsedUserID }),
		...(actionValue === '' ? {} : { action: actionValue }),
		...(routeValue === '' ? {} : { route: routeValue }),
		...(success.value === '' ? {} : { isSuccess: success.value }),
		...(timeRange.value.length === 0 ? {} : { from: timeRange.value[0], to: timeRange.value[1] }),
	}
}

function search(): void {
	const next = buildQuery(1)
	if (next === null) return
	query.value = next
	void loadLogs()
}

function reset(): void {
	userID.value = ''
	action.value = ''
	route.value = ''
	success.value = ''
	timeRange.value = []
	query.value = { page: 1, pageSize: query.value.pageSize }
	void loadLogs()
}

function changePage(page: number): void {
	query.value = { ...query.value, page }
	void loadLogs()
}

function changePageSize(pageSize: number): void {
	query.value = { ...query.value, page: 1, pageSize }
	void loadLogs()
}

function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) { changePageSize(next.pageSize); return }
  changePage(next.currentPage)
}

function formatTime(value: string): string {
	return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

function formatJSON(value: unknown): string {
	if (value === null) return '-'
	return JSON.stringify(value, null, 2) ?? '-'
}

function methodTagType(method: string): 'primary' | 'success' | 'warning' | 'danger' | 'info' {
	if (method === 'POST') return 'success'
	if (method === 'PUT' || method === 'PATCH') return 'warning'
	if (method === 'DELETE') return 'danger'
	if (method === 'GET') return 'primary'
	return 'info'
}

onMounted(() => { void loadLogs() })
</script>

<template>
	<section class="operation-log-page management-page">
		<Search
			v-model="searchModel"
			class="operation-log-filters management-page__filters"
			:fields="searchFields"
			:query-label="t('operationLog.search')"
			:reset-label="t('operationLog.reset')"
			query-test-id="operation-log-search"
			reset-test-id="operation-log-reset"
			@query="search"
			@reset="reset"
		/>

			<el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
			<div v-if="loading" data-testid="operation-log-loading" :aria-label="t('operationLog.loading')">
				<el-skeleton :rows="6" animated />
			</div>
			<AppTable
				v-else
				data-testid="operation-log-table"
				:columns="tableColumns"
				:data="rows"
				:pagination="tablePagination"
				result-state="success"
				:aria-label="t('operationLog.title')"
				:refresh-label="t('operationLog.refresh')"
				@refresh="loadLogs"
				@update:pagination="updateTablePagination"
			>
				<template #expand="{ row }: { row: OperationLogItem }">
						<div class="operation-log-detail">
						<h2>{{ t('operationLog.detailTitle') }}</h2>
						<dl>
							<div><dt>{{ t('operationLog.requestId') }}</dt><dd><code>{{ row.requestId }}</code></dd></div>
							<div><dt>{{ t('operationLog.platform') }}</dt><dd>{{ row.platform }}</dd></div>
							<div><dt>{{ t('operationLog.userAgent') }}</dt><dd>{{ row.userAgent }}</dd></div>
						</dl>
						<div class="operation-log-summaries">
							<section><h3>{{ t('operationLog.detail.request') }}</h3><pre>{{ formatJSON(row.requestData) }}</pre></section>
							<section><h3>{{ t('operationLog.detail.response') }}</h3><pre>{{ formatJSON(row.responseData) }}</pre></section>
						</div>
					</div>
				</template>
				<template #cell-method="{ row }: { row: OperationLogItem }"><el-tag :type="methodTagType(row.method)" effect="plain">{{ row.method }}</el-tag></template>
				<template #cell-user="{ row }: { row: OperationLogItem }">{{ row.userId === null ? '-' : `#${row.userId}` }}</template>
				<template #cell-status="{ row }: { row: OperationLogItem }"><el-tag :type="row.isSuccess === YesNo.Yes ? 'success' : 'danger'" effect="light">{{ row.statusCode }}</el-tag></template>
				<template #cell-latency="{ row }: { row: OperationLogItem }">{{ row.latencyMs }} ms</template>
				<template #cell-createdAt="{ row }: { row: OperationLogItem }">{{ formatTime(row.createdAt) }}</template>
				<template #empty><el-empty :description="t('operationLog.empty')" /></template>
			</AppTable>
	</section>
</template>

<style scoped>
.operation-log-page { min-width: 0; }
.operation-log-filters { display: flex; align-items: center; gap: 12px; }
.operation-log-filters { flex-wrap: wrap; }
.operation-log-filters .el-input { width: 190px; }
.operation-log-filters .el-select-v2 { width: 130px; }
.operation-log-detail { padding: 4px 24px 18px; }
.operation-log-detail h2 { margin: 0 0 12px; font-size: 16px; }
.operation-log-detail dl { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 0 0 16px; }
.operation-log-detail dl div { min-width: 0; }
.operation-log-detail dt { color: var(--el-text-color-secondary); font-size: 12px; }
.operation-log-detail dd { margin: 4px 0 0; overflow-wrap: anywhere; }
.operation-log-summaries { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.operation-log-summaries h3 { margin: 0 0 8px; font-size: 14px; }
.operation-log-summaries pre { min-height: 88px; margin: 0; padding: 12px; overflow: auto; border: 1px solid var(--el-border-color-light); border-radius: 4px; background: var(--el-fill-color-light); white-space: pre-wrap; overflow-wrap: anywhere; }
.el-pagination { justify-content: flex-end; }
@media (max-width: 900px) {
	.operation-log-filters { align-items: stretch; flex-direction: column; }
	.operation-log-filters .el-input, .operation-log-filters .el-select-v2 { width: 100%; }
	.operation-log-detail dl, .operation-log-summaries { grid-template-columns: 1fr; }
}
</style>
