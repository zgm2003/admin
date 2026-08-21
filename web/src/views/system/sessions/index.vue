<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { getSessions, getSessionStats, revokeSession, revokeSessions } from '../../../api/session'
import type { SessionItem, SessionListQuery, SessionStats, SessionStatus } from '../../../api/session.contract'
import { useAccessStore } from '../../../store/access'

const { t } = useI18n()
const access = useAccessStore()

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

const canRevoke = computed(() => access.hasPermission('system:session:revoke'))
const platformStats = computed(() => Object.entries(stats.value.platforms).sort(([left], [right]) => left.localeCompare(right)))

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

function selectable(row: SessionItem): boolean {
	return canRevoke.value && row.status === 'active' && !row.isCurrent
}

function selectionChanged(selection: SessionItem[]): void {
	selectedIDs.value = selection.filter(selectable).slice(0, 100).map((row) => row.id)
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
		await ElMessageBox.confirm(t('session.revokeConfirm'), t('session.batchRevoke'), { type: 'warning' })
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
	return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
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
	<section class="session-page">
		<header class="session-toolbar">
			<h1>{{ t('session.title') }}</h1>
			<div class="session-toolbar-actions">
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
				<el-button :icon="Refresh" :loading="listLoading || statsLoading" @click="reloadAuthoritativeData">
					{{ t('session.refresh') }}
				</el-button>
			</div>
		</header>

		<div class="session-stats" v-loading="statsLoading">
			<div class="session-stat-primary">
				<span>{{ t('session.activeTotal') }}</span>
				<strong>{{ stats.activeTotal }}</strong>
			</div>
			<div class="session-platform-stats">
				<span class="session-platform-title">{{ t('session.platformDistribution') }}</span>
				<el-tag v-for="([code, count]) in platformStats" :key="code" effect="plain">{{ code }} · {{ count }}</el-tag>
			</div>
		</div>
		<el-alert v-if="statsError" :title="statsError" type="error" :closable="false" show-icon />

		<div class="session-filters">
			<el-input v-model="username" data-testid="session-username" clearable :placeholder="t('session.username')" @keyup.enter="search" />
			<el-input v-model="platform" data-testid="session-platform" clearable :placeholder="t('session.platform')" @keyup.enter="search" />
			<el-select v-model="status" data-testid="session-status" :placeholder="t('session.statusLabel')" clearable>
				<el-option :label="t('session.status.active')" value="active" />
				<el-option :label="t('session.status.expired')" value="expired" />
				<el-option :label="t('session.status.revoked')" value="revoked" />
			</el-select>
			<el-button data-testid="session-search" type="primary" :icon="Search" @click="search">{{ t('session.search') }}</el-button>
			<el-button @click="reset">{{ t('session.reset') }}</el-button>
		</div>

		<el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
		<el-alert v-if="mutationError" :title="mutationError" type="error" :closable="false" show-icon />
		<div v-if="listLoading" data-testid="session-loading" :aria-label="t('session.loading')">
			<el-skeleton :rows="6" animated />
		</div>
		<el-empty v-else-if="rows.length === 0" :description="t('session.empty')" />
		<el-table
			v-else
			:data="rows"
			data-testid="session-table"
			row-key="id"
			stripe
			@selection-change="selectionChanged"
		>
			<el-table-column v-if="canRevoke" type="selection" width="48" :selectable="selectable" />
			<el-table-column :label="t('session.column.user')" min-width="150">
				<template #default="{ row }: { row: SessionItem }">
					<strong>{{ row.username }}</strong>
					<small>#{{ row.userId }}</small>
				</template>
			</el-table-column>
			<el-table-column prop="platform" :label="t('session.column.platform')" min-width="110" />
			<el-table-column :label="t('session.column.device')" min-width="180">
				<template #default="{ row }: { row: SessionItem }"><code>{{ row.deviceId }}</code></template>
			</el-table-column>
			<el-table-column prop="clientIp" :label="t('session.column.ip')" min-width="130" />
			<el-table-column prop="userAgent" :label="t('session.column.userAgent')" min-width="180" show-overflow-tooltip />
			<el-table-column :label="t('session.column.createdAt')" min-width="180">
				<template #default="{ row }: { row: SessionItem }">{{ formatTime(row.createdAt) }}</template>
			</el-table-column>
			<el-table-column :label="t('session.column.expiresAt')" min-width="180">
				<template #default="{ row }: { row: SessionItem }">{{ formatTime(row.refreshExpiresAt) }}</template>
			</el-table-column>
			<el-table-column :label="t('session.statusLabel')" width="100">
				<template #default="{ row }: { row: SessionItem }">
					<el-tag :type="statusTagType(row.status)" effect="light">{{ t(`session.status.${row.status}`) }}</el-tag>
				</template>
			</el-table-column>
			<el-table-column :label="t('session.column.actions')" fixed="right" width="120">
				<template #default="{ row }: { row: SessionItem }">
					<el-tag v-if="row.isCurrent" type="primary" effect="plain">{{ t('session.current') }}</el-tag>
					<el-button
						v-else-if="canRevoke && row.status === 'active'"
						:data-testid="`session-revoke-${row.id}`"
						type="danger"
						link
						:loading="mutating"
						@click="revokeOne(row)"
					>
						{{ t('session.revoke') }}
					</el-button>
				</template>
			</el-table-column>
		</el-table>

		<el-pagination
			v-if="total > 0"
			data-testid="session-pagination"
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
.session-page { display: flex; min-width: 0; flex-direction: column; gap: 16px; }
.session-toolbar, .session-toolbar-actions, .session-filters, .session-stats, .session-platform-stats { display: flex; align-items: center; gap: 12px; }
.session-toolbar { justify-content: space-between; }
.session-toolbar h1 { margin: 0; font-size: 22px; }
.session-stats { min-height: 72px; padding: 12px 16px; border: 1px solid var(--el-border-color-light); border-radius: 6px; }
.session-stat-primary { display: flex; min-width: 140px; flex-direction: column; gap: 4px; }
.session-stat-primary span, .session-platform-title, small { color: var(--el-text-color-secondary); font-size: 12px; }
.session-stat-primary strong { font-size: 24px; }
.session-platform-stats { flex-wrap: wrap; }
.session-filters .el-input { width: 220px; }
.session-filters .el-select { width: 150px; }
.el-table strong, .el-table small { display: block; }
.el-pagination { justify-content: flex-end; }
@media (max-width: 900px) {
	.session-toolbar, .session-filters, .session-stats { align-items: stretch; flex-direction: column; }
	.session-filters .el-input, .session-filters .el-select { width: 100%; }
}
</style>
