<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { getOperationLogs } from '../../../api/operation-log'
import type { OperationLogItem, OperationLogListQuery } from '../../../api/operation-log.contract'
import { YesNo } from '../../../enums/yes-no'

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
	<section class="operation-log-page">
		<header class="operation-log-toolbar">
			<h1>{{ t('operationLog.title') }}</h1>
			<el-button :icon="Refresh" :loading="loading" @click="loadLogs">{{ t('operationLog.refresh') }}</el-button>
		</header>

		<div class="operation-log-filters">
			<el-input v-model="userID" data-testid="operation-log-user-id" clearable :placeholder="t('operationLog.userId')" @keyup.enter="search" />
			<el-input v-model="action" data-testid="operation-log-action" clearable :placeholder="t('operationLog.action')" @keyup.enter="search" />
			<el-input v-model="route" data-testid="operation-log-route" clearable :placeholder="t('operationLog.route')" @keyup.enter="search" />
			<el-select v-model="success" data-testid="operation-log-success" clearable :placeholder="t('operationLog.successLabel')">
				<el-option :label="t('operationLog.all')" value="" />
				<el-option :label="t('operationLog.success.yes')" :value="YesNo.Yes" />
				<el-option :label="t('operationLog.success.no')" :value="YesNo.No" />
			</el-select>
			<el-date-picker
				v-model="timeRange"
				data-testid="operation-log-time"
				type="datetimerange"
				value-format="YYYY-MM-DDTHH:mm:ssZ"
				:range-separator="'-'"
				:start-placeholder="t('operationLog.timeRange')"
				:end-placeholder="t('operationLog.timeRange')"
			/>
			<el-button data-testid="operation-log-search" type="primary" :icon="Search" @click="search">{{ t('operationLog.search') }}</el-button>
			<el-button @click="reset">{{ t('operationLog.reset') }}</el-button>
		</div>

		<el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
		<div v-if="loading" data-testid="operation-log-loading" :aria-label="t('operationLog.loading')">
			<el-skeleton :rows="6" animated />
		</div>
		<el-empty v-else-if="rows.length === 0" :description="t('operationLog.empty')" />
		<el-table v-else :data="rows" data-testid="operation-log-table" row-key="id" stripe>
			<el-table-column type="expand">
				<template #default="{ row }: { row: OperationLogItem }">
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
			</el-table-column>
			<el-table-column :label="t('operationLog.column.method')" width="90">
				<template #default="{ row }: { row: OperationLogItem }"><el-tag :type="methodTagType(row.method)" effect="plain">{{ row.method }}</el-tag></template>
			</el-table-column>
			<el-table-column prop="action" :label="t('operationLog.column.action')" min-width="160" show-overflow-tooltip />
			<el-table-column prop="route" :label="t('operationLog.column.route')" min-width="220" show-overflow-tooltip />
			<el-table-column :label="t('operationLog.column.user')" width="100">
				<template #default="{ row }: { row: OperationLogItem }">{{ row.userId === null ? '-' : `#${row.userId}` }}</template>
			</el-table-column>
			<el-table-column prop="clientIp" :label="t('operationLog.column.ip')" min-width="130" />
			<el-table-column :label="t('operationLog.column.status')" width="100">
				<template #default="{ row }: { row: OperationLogItem }">
					<el-tag :type="row.isSuccess === YesNo.Yes ? 'success' : 'danger'" effect="light">{{ row.statusCode }}</el-tag>
				</template>
			</el-table-column>
			<el-table-column :label="t('operationLog.column.latency')" width="100">
				<template #default="{ row }: { row: OperationLogItem }">{{ row.latencyMs }} ms</template>
			</el-table-column>
			<el-table-column :label="t('operationLog.column.createdAt')" min-width="180">
				<template #default="{ row }: { row: OperationLogItem }">{{ formatTime(row.createdAt) }}</template>
			</el-table-column>
		</el-table>

		<el-pagination
			v-if="total > 0"
			data-testid="operation-log-pagination"
			background
			layout="total, sizes, prev, pager, next"
			:current-page="query.page"
			:page-size="query.pageSize"
			:page-sizes="[20, 50, 100]"
			:total="total"
			@current-change="changePage"
			@size-change="changePageSize"
		/>
	</section>
</template>

<style scoped>
.operation-log-page { display: flex; min-width: 0; flex-direction: column; gap: 16px; }
.operation-log-toolbar, .operation-log-filters { display: flex; align-items: center; gap: 12px; }
.operation-log-toolbar { justify-content: space-between; }
.operation-log-toolbar h1 { margin: 0; font-size: 22px; }
.operation-log-filters { flex-wrap: wrap; }
.operation-log-filters .el-input { width: 190px; }
.operation-log-filters .el-select { width: 130px; }
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
	.operation-log-toolbar, .operation-log-filters { align-items: stretch; flex-direction: column; }
	.operation-log-filters .el-input, .operation-log-filters .el-select { width: 100%; }
	.operation-log-detail dl, .operation-log-summaries { grid-template-columns: 1fr; }
}
</style>
