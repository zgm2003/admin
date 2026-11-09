<!-- 页面：工作台 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowRight, Check, Refresh, Warning } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { getHealth, getReadiness } from '@/api/health'
import { ProtocolError } from '@/types/http'
import AppServerOverview from '@/components/AppServerOverview/index.vue'
import { usePermissionStore } from '@/store/permission'
import { useBrandStore } from '@/store/brand'

type StatusState = 'checking' | 'up' | 'error'

const { t, locale } = useI18n()
const router = useRouter()
const permission = usePermissionStore()
const brand = useBrandStore()
const projectName = computed(
  () =>
    (locale.value === 'en-US' ? brand.settings.titleEnUS : brand.settings.titleZhCN) ||
    t('navigation.admin'),
)
const serverRefreshKey = ref(0)
const operations = computed(() =>
  [
    { code: 'system:server:view', label: 'navigation.systemServer' },
    { code: 'system:queueMonitor:view', label: 'navigation.systemQueueMonitor' },
    { code: 'system:scheduler:view', label: 'navigation.systemScheduler' },
    { code: 'system:configCacheStatus:view', label: 'navigation.systemConfigCacheStatus' },
    { code: 'system:operationLog:view', label: 'navigation.systemOperationLog' },
    { code: 'system:setting:view', label: 'navigation.systemSetting' },
  ].filter((item) => permission.hasPermission(item.code) && router.hasRoute(`access:${item.code}`)),
)
const architecture = computed(() => [
  { name: 'Vue 3 · TypeScript', text: t('dashboard.architecture.web') },
  { name: '/api', text: t('dashboard.architecture.proxy') },
  { name: 'Go API · Worker', text: t('dashboard.architecture.server') },
  { name: 'PostgreSQL · Redis', text: t('dashboard.architecture.data') },
])

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
  if (refreshing.value) return
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

function refreshAll(): void {
  if (refreshing.value) return
  serverRefreshKey.value += 1
  void refreshHealth()
}

onMounted(refreshHealth)
</script>

<template>
  <div class="dashboard-page">
    <div class="admin-content">
      <div class="dashboard">
        <div class="dashboard-intro">
          <div>
            <span class="section-kicker">{{ t('dashboard.overview') }}</span>
            <h2>{{ projectName }}</h2>
          </div>
          <div class="dashboard-intro__side">
            <p>{{ t('dashboard.subtitle') }}</p>
            <el-button
              class="refresh-button"
              :loading="refreshing"
              :icon="Refresh"
              :title="t('dashboard.refresh')"
              :aria-label="t('dashboard.refresh')"
              data-testid="dashboard-refresh"
              @click="refreshAll"
            >
              {{ t('dashboard.refresh') }}
            </el-button>
          </div>
        </div>
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

        <AppServerOverview :refresh-key="serverRefreshKey" />

        <section
          v-if="operations.length"
          class="operations-panel"
          data-testid="operations-links"
          :aria-label="t('dashboard.operations')"
        >
          <h2>{{ t('dashboard.operations') }}</h2>
          <div class="operations-links">
            <RouterLink
              v-for="item in operations"
              :key="item.code"
              :to="{ name: `access:${item.code}` }"
            >
              <span>{{ t(item.label) }}</span
              ><el-icon><ArrowRight /></el-icon>
            </RouterLink>
          </div>
        </section>

        <section
          class="project-panel"
          data-testid="project-introduction"
          aria-labelledby="project-title"
        >
          <header>
            <span class="section-kicker">{{ t('dashboard.project') }}</span>
            <h2 id="project-title">{{ t('dashboard.projectTitle') }}</h2>
            <p>{{ t('dashboard.projectDescription') }}</p>
          </header>
          <div class="project-architecture">
            <div
              v-for="(item, index) in architecture"
              :key="item.name"
              class="project-architecture__node"
            >
              <code>{{ item.name }}</code>
              <p>{{ item.text }}</p>
              <el-icon v-if="index < architecture.length - 1" class="project-architecture__arrow"
                ><ArrowRight
              /></el-icon>
            </div>
          </div>
          <div class="project-principles">
            <p>{{ t('dashboard.capabilities') }}</p>
            <p>{{ t('dashboard.permissions') }}</p>
          </div>
          <details class="project-deployment">
            <summary>{{ t('dashboard.deployment') }}</summary>
            <p>{{ t('dashboard.deploymentDescription') }}</p>
            <code>/api → Go API</code>
          </details>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.dashboard-intro {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;

  h2 {
    font: 650 24px/1.4 var(--admin-font-display);
    letter-spacing: -0.5px;
  }

  &__side {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    max-width: 560px;
    gap: 16px;

    > p {
      margin: 0;
      color: var(--admin-text-muted);
      font-size: 12px;
      line-height: 1.8;
      text-align: right;
    }
  }
}
.operations-panel,
.project-panel {
  min-width: 0;
  padding: 24px;
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-lg);
}
.operations-panel h2 {
  margin-bottom: 16px;
}
.operations-links {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  a {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 14px 16px;
    color: var(--admin-text-soft);
    background: var(--admin-surface-soft);
    border: 1px solid transparent;
    border-radius: var(--admin-radius-sm);
    font-size: 13px;
    text-decoration: none;
    transition:
      border-color var(--admin-motion-fast),
      color var(--admin-motion-fast);
    &:hover {
      color: var(--el-color-primary);
      border-color: var(--el-color-primary-light-7);
    }
    &:focus-visible {
      outline: 2px solid var(--el-color-primary);
      outline-offset: 2px;
    }
  }
}
.project-panel {
  header > p {
    max-width: 660px;
    margin: 10px 0 0;
    color: var(--admin-text-soft);
    font-size: 12px;
    line-height: 1.9;
  }
}
.project-architecture {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 24px 0;
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-md);
  background: var(--admin-surface-soft);
  &__node {
    position: relative;
    padding: 22px 20px;
    min-width: 0;
    & + & {
      border-left: 1px solid var(--admin-border);
    }
    code {
      color: var(--el-color-primary);
      font: 600 12px/1.7 var(--admin-font-data);
      overflow-wrap: anywhere;
    }
    p {
      margin: 8px 0 0;
      color: var(--admin-text-muted);
      font-size: 11px;
      line-height: 1.7;
    }
  }
  &__arrow {
    position: absolute;
    top: 26px;
    right: -9px;
    z-index: 1;
    padding: 2px;
    color: var(--admin-text-muted);
    background: var(--admin-surface-soft);
  }
}
.project-principles {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  p {
    margin: 0;
    color: var(--admin-text-soft);
    font-size: 12px;
    line-height: 1.9;
  }
}
.project-deployment {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid var(--admin-border);
  color: var(--admin-text-soft);
  font-size: 12px;
  summary {
    width: fit-content;
    cursor: pointer;
    color: var(--admin-text);
    line-height: 1.7;
    &:focus-visible {
      outline: 2px solid var(--el-color-primary);
      outline-offset: 3px;
    }
  }
  p {
    line-height: 1.9;
  }
  > code {
    display: inline-block;
    padding: 6px 10px;
    background: var(--admin-surface-soft);
    border-radius: var(--admin-radius-sm);
    font-family: var(--admin-font-data);
  }
}
@media (max-width: 840px) {
  .dashboard-intro {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;

    &__side {
      width: 100%;
      max-width: none;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;

      > p {
        text-align: left;
      }
    }
  }
  .project-architecture {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    &__node:nth-child(3) {
      border-left: 0;
    }
    &__node:nth-child(n + 3) {
      border-top: 1px solid var(--admin-border);
    }
    &__arrow {
      display: none;
    }
  }
  .operations-links {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 480px) {
  .dashboard-intro__side {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .operations-panel,
  .project-panel {
    padding: 20px 16px;
  }
  .project-architecture {
    grid-template-columns: 1fr;
    &__node {
      padding: 16px;
      & + & {
        border-left: 0;
        border-top: 1px solid var(--admin-border);
      }
    }
  }
  .project-principles,
  .operations-links {
    grid-template-columns: 1fr;
    gap: 12px;
  }
}
</style>
