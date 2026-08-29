<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Check, Monitor, Refresh, Warning } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { getHealth, getReadiness } from '../../api/health'
import { ProtocolError } from '../../types/http'
import ReadinessChart from './components/ReadinessChart.vue'

type StatusState = 'checking' | 'up' | 'error'

const { t } = useI18n()

const apiStatus = ref<StatusState>('checking')
const postgresqlStatus = ref<StatusState>('checking')
const redisStatus = ref<StatusState>('checking')
const healthError = ref('')
const refreshing = ref(false)

function statusText(status: StatusState): string {
  if (status === 'up') return t('dashboard.status.up')
  if (status === 'error') return t('dashboard.status.error')
  return t('dashboard.status.checking')
}

function statusIcon(status: StatusState) {
  return status === 'up' ? Check : status === 'error' ? Warning : Refresh
}

function errorMessage(error: unknown): string {
  if (error instanceof ProtocolError) return t('request.protocolError')
  return error instanceof Error ? error.message : t('dashboard.unknownError')
}

async function refreshHealth(): Promise<void> {
  refreshing.value = true
  healthError.value = ''
  apiStatus.value = 'checking'
  postgresqlStatus.value = 'checking'
  redisStatus.value = 'checking'

  const [apiResult, readinessResult] = await Promise.allSettled([getHealth(), getReadiness()])

  if (apiResult.status === 'fulfilled' && apiResult.value.status === 'up') {
    apiStatus.value = 'up'
  } else {
    apiStatus.value = 'error'
    healthError.value = errorMessage(
      apiResult.status === 'rejected' ? apiResult.reason : new Error(t('dashboard.healthInvalid')),
    )
  }

  if (readinessResult.status === 'fulfilled') {
    postgresqlStatus.value = readinessResult.value.postgresql === 'up' ? 'up' : 'error'
    redisStatus.value = readinessResult.value.redis === 'up' ? 'up' : 'error'
  } else {
    postgresqlStatus.value = 'error'
    redisStatus.value = 'error'
    healthError.value = errorMessage(readinessResult.reason)
  }
  refreshing.value = false
}

onMounted(refreshHealth)
</script>

<template>
  <div class="dashboard-page">
    <section class="dashboard-toolbar" aria-labelledby="dashboard-title">
      <div class="dashboard-toolbar__title">
        <span class="dashboard-toolbar__icon"><el-icon><Monitor /></el-icon></span>
        <div>
          <span class="dashboard-toolbar__eyebrow">{{ t('dashboard.eyebrow') }}</span>
          <h1 id="dashboard-title">{{ t('dashboard.title') }}</h1>
        </div>
      </div>

      <el-button
        class="refresh-button"
        :loading="refreshing"
        :icon="Refresh"
        :title="t('dashboard.refresh')"
        :aria-label="t('dashboard.refresh')"
        @click="refreshHealth"
      >
        {{ t('dashboard.refresh') }}
      </el-button>
    </section>

    <div class="admin-content">
      <div class="dashboard">
        <p v-if="healthError" class="inline-error health-error" data-testid="health-error">
          {{ healthError }}
        </p>

        <section
          class="status-track"
          data-testid="dashboard-summary"
          :aria-label="t('dashboard.liveStatus')"
          aria-live="polite"
        >
          <div
            class="status-track__item"
            :data-state="apiStatus"
            :aria-label="`${t('dashboard.api')}: ${statusText(apiStatus)}`"
            data-testid="api-status"
          >
            <el-icon><component :is="statusIcon(apiStatus)" /></el-icon>
            <span class="status-track__name">{{ t('dashboard.api') }}</span>
            <strong>{{ statusText(apiStatus) }}</strong>
          </div>
          <div
            class="status-track__item"
            :data-state="postgresqlStatus"
            :aria-label="`${t('dashboard.postgresql')}: ${statusText(postgresqlStatus)}`"
            data-testid="postgresql-status"
          >
            <el-icon><component :is="statusIcon(postgresqlStatus)" /></el-icon>
            <span class="status-track__name">
              <span class="status-track__label-full">{{ t('dashboard.postgresql') }}</span>
              <span class="status-track__label-short" aria-hidden="true">PG</span>
            </span>
            <strong>{{ statusText(postgresqlStatus) }}</strong>
          </div>
          <div
            class="status-track__item"
            :data-state="redisStatus"
            :aria-label="`${t('dashboard.redis')}: ${statusText(redisStatus)}`"
            data-testid="redis-status"
          >
            <el-icon><component :is="statusIcon(redisStatus)" /></el-icon>
            <span class="status-track__name">{{ t('dashboard.redis') }}</span>
            <strong>{{ statusText(redisStatus) }}</strong>
          </div>
        </section>

        <div class="dashboard-grid dashboard-grid--single">
          <section class="tool-panel readiness-panel">
            <header class="tool-panel__header">
              <div>
                <span class="section-kicker">DEPENDENCIES</span>
                <h2>{{ t('dashboard.dependencies') }}</h2>
              </div>
              <el-tag size="small" effect="plain" type="success">{{ t('dashboard.live') }}</el-tag>
            </header>
            <ReadinessChart
              :api="apiStatus"
              :postgresql="postgresqlStatus"
              :redis="redisStatus"
            />
          </section>

        </div>
      </div>
    </div>
  </div>
</template>
