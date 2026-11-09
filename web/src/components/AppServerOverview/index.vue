<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Cpu, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { getServerSnapshot, type ServerSnapshot } from '@/api/system/server'
import { ProtocolError } from '@/types/http'

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 })
const { t, locale } = useI18n()
const snapshot = ref<ServerSnapshot | null>(null)
const loading = ref(false)
const error = ref('')
let generation = 0

function bytes(value: number): string {
  if (value < 1024) return `${value} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)) - 1, units.length - 1)
  return `${(value / 1024 ** (index + 1)).toLocaleString(locale.value, { maximumFractionDigits: 1 })} ${units[index]}`
}

function duration(seconds: number): string {
  return t('system.server.duration', {
    days: Math.floor(seconds / 86400),
    hours: Math.floor((seconds % 86400) / 3600),
    minutes: Math.floor((seconds % 3600) / 60),
  })
}

const resources = computed(() =>
  snapshot.value
    ? [
        {
          key: 'cpu',
          title: t('system.server.cpu'),
          percent: snapshot.value.cpuPercent,
          description: t('system.server.cores', { count: snapshot.value.logicalCores }),
        },
        {
          key: 'memory',
          title: t('system.server.memory'),
          percent: snapshot.value.memory.usedPercent,
          description: t('system.server.capacity', {
            available: bytes(snapshot.value.memory.availableBytes),
            total: bytes(snapshot.value.memory.totalBytes),
          }),
        },
        {
          key: 'disk',
          title: t('system.server.disk'),
          percent: snapshot.value.disk.usedPercent,
          description: t('system.server.capacity', {
            available: bytes(snapshot.value.disk.availableBytes),
            total: bytes(snapshot.value.disk.totalBytes),
          }),
        },
      ]
    : [],
)

const sampledAt = computed(() =>
  snapshot.value ? new Date(snapshot.value.sampledAt).toLocaleString(locale.value) : '',
)

async function refresh(): Promise<void> {
  if (loading.value) return
  const current = ++generation
  loading.value = true
  error.value = ''
  snapshot.value = null
  try {
    const result = await getServerSnapshot()
    if (generation === current) snapshot.value = result
  } catch (reason: unknown) {
    if (generation === current)
      error.value =
        reason instanceof ProtocolError
          ? t('request.protocolError')
          : reason instanceof Error
            ? reason.message
            : t('system.server.loadFailed')
  } finally {
    if (generation === current) loading.value = false
  }
}

watch(
  () => props.refreshKey,
  () => {
    void refresh()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  generation += 1
})
</script>

<template>
  <section class="server-panel" :aria-label="t('system.server.title')" :aria-busy="loading">
    <header class="server-panel__header">
      <div class="server-panel__heading">
        <span class="server-panel__icon"
          ><el-icon><Cpu /></el-icon
        ></span>
        <div>
          <h2>{{ t('system.server.title') }}</h2>
          <p>{{ t('system.server.scope') }}</p>
        </div>
      </div>
      <el-button data-testid="server-refresh" :icon="Refresh" :loading="loading" @click="refresh">
        {{ t('common.refresh') }}
      </el-button>
    </header>

    <p v-if="loading" class="server-panel__notice" role="status">
      {{ t('system.server.loading') }}
    </p>
    <p v-else-if="error" class="server-panel__error" role="status" data-testid="server-error">
      {{ t('system.server.loadFailed') }} · {{ error }}
    </p>

    <template v-else-if="snapshot">
      <div class="server-resources" data-testid="server-metrics">
        <div v-for="resource in resources" :key="resource.key" class="server-resource">
          <span class="server-resource__label">{{ resource.title }}</span>
          <strong>{{ resource.percent.toFixed(1) }}<small>%</small></strong>
          <el-progress
            :percentage="resource.percent"
            :show-text="false"
            :stroke-width="5"
            :aria-label="resource.title"
          />
          <p>{{ resource.description }}</p>
        </div>
      </div>
      <dl class="server-facts" data-testid="server-host">
        <div>
          <dt>{{ t('system.server.hostname') }}</dt>
          <dd>{{ snapshot.hostname }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.os') }}</dt>
          <dd>{{ snapshot.os }} / {{ snapshot.arch }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.hostUptime') }}</dt>
          <dd>{{ duration(snapshot.hostUptimeSeconds) }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.processUptime') }}</dt>
          <dd>{{ duration(snapshot.processUptimeSeconds) }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.runtime') }}</dt>
          <dd>{{ snapshot.goVersion }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.goroutines') }}</dt>
          <dd>{{ snapshot.goroutines }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.heap') }}</dt>
          <dd>{{ bytes(snapshot.heapAllocBytes) }}</dd>
        </div>
        <div>
          <dt>{{ t('system.server.diskPath') }}</dt>
          <dd>{{ snapshot.diskPath }}</dd>
        </div>
      </dl>
      <footer class="server-panel__footer">
        <span
          >{{ t('system.server.sampledAt') }}
          <time :datetime="snapshot.sampledAt">{{ sampledAt }}</time></span
        >
        <span>{{ t('system.server.manualRefresh') }}</span>
      </footer>
    </template>
  </section>
</template>

<style scoped lang="scss">
.server-panel {
  min-width: 0;
  overflow: hidden;
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-lg);
  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 22px 24px;
    border-bottom: 1px solid var(--admin-border);
  }
  &__heading {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: 12px;
    h2 {
      margin: 0;
      font-size: 16px;
    }
    p {
      margin: 5px 0 0;
      color: var(--admin-text-muted);
      font-size: 12px;
      line-height: 1.6;
    }
  }
  &__icon {
    display: grid;
    place-items: center;
    flex: 0 0 38px;
    height: 38px;
    color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
    border-radius: var(--admin-radius-sm);
    font-size: 21px;
  }
  &__notice,
  &__error {
    padding: 24px;
    margin: 0;
    color: var(--admin-text-soft);
    line-height: 1.8;
    overflow-wrap: anywhere;
  }
  &__error {
    color: var(--el-color-danger);
  }
  &__footer {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;
    padding: 14px 24px;
    color: var(--admin-text-muted);
    background: var(--admin-surface-soft);
    font-size: 11px;
    line-height: 1.6;
  }
}
.server-resources {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 24px 0;
}
.server-resource {
  min-width: 0;
  padding: 0 24px;
  & + & {
    border-left: 1px solid var(--admin-border);
  }
  &__label {
    color: var(--admin-text-soft);
    font-size: 12px;
  }
  strong {
    display: block;
    margin: 10px 0 16px;
    font: 500 34px/1.1 var(--admin-font-data);
    letter-spacing: -1.5px;
    small {
      margin-left: 4px;
      font-size: 17px;
      color: var(--admin-text-muted);
    }
  }
  p {
    margin: 12px 0 0;
    color: var(--admin-text-muted);
    font-size: 11px;
    line-height: 1.8;
  }
}
.server-facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 0;
  padding: 20px 24px;
  gap: 22px;
  border-top: 1px solid var(--admin-border);
  dt {
    color: var(--admin-text-muted);
    font-size: 11px;
  }
  dd {
    margin: 7px 0 0;
    color: var(--admin-text);
    font: 12px/1.7 var(--admin-font-data);
    overflow-wrap: anywhere;
  }
}
@media (max-width: 960px) {
  .server-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 600px) {
  .server-panel__header {
    padding: 18px 16px;
    align-items: flex-start;
  }
  .server-panel__icon {
    display: none;
  }
  .server-resources {
    grid-template-columns: 1fr;
    padding: 0 16px;
  }
  .server-resource {
    padding: 20px 0;
    & + & {
      border-left: 0;
      border-top: 1px solid var(--admin-border);
    }
    strong {
      font-size: 30px;
    }
  }
  .server-facts {
    padding: 20px 16px;
    gap: 18px;
  }
  .server-panel__footer {
    padding: 14px 16px;
  }
}
</style>
