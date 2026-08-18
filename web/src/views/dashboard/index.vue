<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Check, Monitor, Refresh, Right, Warning } from '@element-plus/icons-vue'

import { getHealth, getReadiness } from '../../api/health'
import { createExampleTask } from '../../api/taskDemo'
import ReadinessChart from './components/ReadinessChart.vue'

type StatusState = 'checking' | 'up' | 'error'

const apiStatus = ref<StatusState>('checking')
const postgresqlStatus = ref<StatusState>('checking')
const redisStatus = ref<StatusState>('checking')
const healthError = ref('')
const refreshing = ref(false)

const message = ref('')
const submitting = ref(false)
const taskID = ref('')
const taskError = ref('')

const canSubmit = computed(() => {
  const length = Array.from(message.value.trim()).length
  return length > 0 && length <= 200 && !submitting.value
})

function statusText(status: StatusState): string {
  if (status === 'up') return '运行正常'
  if (status === 'error') return '检查失败'
  return '检查中'
}

function statusIcon(status: StatusState) {
  return status === 'up' ? Check : status === 'error' ? Warning : Refresh
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '状态检查返回未知错误'
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
      apiResult.status === 'rejected' ? apiResult.reason : new Error('API 状态无效'),
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

async function submitTask(): Promise<void> {
  if (!canSubmit.value) return
  submitting.value = true
  taskID.value = ''
  taskError.value = ''
  try {
    const created = await createExampleTask({ message: message.value.trim() })
    taskID.value = created.taskId
  } catch (error) {
    taskError.value = errorMessage(error)
  } finally {
    submitting.value = false
  }
}

onMounted(refreshHealth)
</script>

<template>
  <div class="dashboard-page">
    <section class="dashboard-toolbar" aria-labelledby="dashboard-title">
      <div class="dashboard-toolbar__title">
        <span class="dashboard-toolbar__icon"><el-icon><Monitor /></el-icon></span>
        <div>
          <span class="dashboard-toolbar__eyebrow">SYSTEM OVERVIEW</span>
          <h1 id="dashboard-title">工作台</h1>
        </div>
      </div>

      <el-button
        class="refresh-button"
        :loading="refreshing"
        :icon="Refresh"
        title="刷新状态"
        aria-label="刷新状态"
        @click="refreshHealth"
      >
        刷新状态
      </el-button>
    </section>

    <div class="admin-content">
      <div class="dashboard">
        <p v-if="healthError" class="inline-error health-error" data-testid="health-error">
          {{ healthError }}
        </p>

        <section class="status-track" data-testid="dashboard-summary" aria-label="实时状态" aria-live="polite">
          <div
            class="status-track__item"
            :data-state="apiStatus"
            :aria-label="`API：${statusText(apiStatus)}`"
            data-testid="api-status"
          >
            <el-icon><component :is="statusIcon(apiStatus)" /></el-icon>
            <span class="status-track__name">API</span>
            <strong>{{ statusText(apiStatus) }}</strong>
          </div>
          <div
            class="status-track__item"
            :data-state="postgresqlStatus"
            :aria-label="`PostgreSQL：${statusText(postgresqlStatus)}`"
            data-testid="postgresql-status"
          >
            <el-icon><component :is="statusIcon(postgresqlStatus)" /></el-icon>
            <span class="status-track__name">
              <span class="status-track__label-full">PostgreSQL</span>
              <span class="status-track__label-short" aria-hidden="true">PG</span>
            </span>
            <strong>{{ statusText(postgresqlStatus) }}</strong>
          </div>
          <div
            class="status-track__item"
            :data-state="redisStatus"
            :aria-label="`Redis：${statusText(redisStatus)}`"
            data-testid="redis-status"
          >
            <el-icon><component :is="statusIcon(redisStatus)" /></el-icon>
            <span class="status-track__name">Redis</span>
            <strong>{{ statusText(redisStatus) }}</strong>
          </div>
        </section>

        <div class="dashboard-grid">
          <section class="tool-panel readiness-panel">
            <header class="tool-panel__header">
              <div>
                <span class="section-kicker">DEPENDENCIES</span>
                <h2>依赖状态</h2>
              </div>
              <el-tag size="small" effect="plain" type="success">实时</el-tag>
            </header>
            <ReadinessChart
              :api="apiStatus"
              :postgresql="postgresqlStatus"
              :redis="redisStatus"
            />
          </section>

          <section class="tool-panel task-panel">
            <header class="tool-panel__header">
              <div>
                <span class="section-kicker">ASYNQ</span>
                <h2>示例任务</h2>
              </div>
              <el-tag size="small" effect="plain">异步</el-tag>
            </header>

            <form class="task-form" @submit.prevent="submitTask">
              <label for="task-message">消息内容</label>
              <el-input
                id="task-message"
                v-model="message"
                data-testid="task-message"
                maxlength="200"
                show-word-limit
                placeholder="输入任务消息"
              />
              <el-button
                data-testid="task-submit"
                type="primary"
                native-type="submit"
                :icon="Right"
                :loading="submitting"
                :disabled="!canSubmit"
              >
                投递任务
              </el-button>
            </form>

            <div v-if="taskID" class="task-result" data-testid="task-id">
              <span>任务 ID</span>
              <code>{{ taskID }}</code>
            </div>
            <p v-if="taskError" class="inline-error">{{ taskError }}</p>
          </section>
        </div>
      </div>
    </div>
  </div>
</template>
