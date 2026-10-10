<script setup lang="ts">
import { onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { grantQueueMonitor, QUEUE_MONITOR_UI_URL } from '@/api/system/queueMonitor'
import { usePermissionStore } from '@/store/permission'

const { t } = useI18n()
const access = usePermissionStore()
const loading = ref(false)
const loaded = ref(false)
const failed = ref(false)
const frameFailed = ref(false)
const frameLoading = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

function clearRenewal(): void {
  if (timer !== undefined) {
    clearInterval(timer)
    timer = undefined
  }
}
function startRenewal(): void {
  clearRenewal()
  timer = setInterval(() => {
    void grant()
  }, 45_000)
}
async function grant(): Promise<void> {
  if (!access.hasPermission('system:queueMonitor:list')) return
  loading.value = true
  failed.value = false
  try {
    await grantQueueMonitor()
    loaded.value = true
    frameFailed.value = false
    frameLoading.value = true
    startRenewal()
  } catch {
    loaded.value = false
    failed.value = true
    clearRenewal()
  } finally {
    loading.value = false
  }
}
function onFrameError(): void {
  frameLoading.value = false
  frameFailed.value = true
}
function onFrameLoad(): void {
  frameLoading.value = false
}
function onVisibilityChange(): void {
  if (document.hidden) clearRenewal()
  else void grant()
}
onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  void grant()
})
onBeforeUnmount(() => {
  stop()
})
onDeactivated(stop)
function stop(): void {
  clearRenewal()
  document.removeEventListener('visibilitychange', onVisibilityChange)
}
</script>

<template>
  <AppPage class="queue-monitor-page">
    <div v-if="!access.hasPermission('system:queueMonitor:list')" class="queue-monitor-state">
      {{ t('system.queueMonitor.forbidden') }}
    </div>
    <div v-else-if="failed" class="queue-monitor-state">
      <p>{{ t('system.queueMonitor.grantFailed') }}</p>
      <el-button type="primary" :loading="loading" @click="grant">{{
        t('system.queueMonitor.retry')
      }}</el-button>
    </div>
    <div v-else-if="!loaded" class="queue-monitor-state">
      {{ t('system.queueMonitor.loading') }}
    </div>
    <div v-else-if="frameFailed" class="queue-monitor-state">
      <p>{{ t('system.queueMonitor.loadFailed') }}</p>
      <el-button type="primary" :loading="loading" @click="grant">{{
        t('system.queueMonitor.retry')
      }}</el-button>
    </div>
    <div v-else class="queue-monitor-frame-shell">
      <iframe
        class="queue-monitor-frame"
        :src="QUEUE_MONITOR_UI_URL"
        :title="t('system.queueMonitor.title')"
        @error="onFrameError"
        @load="onFrameLoad"
      />
      <div
        v-if="frameLoading"
        class="queue-monitor-frame-loading"
        role="status"
        data-testid="queue-monitor-frame-loading"
      >
        <el-icon class="is-loading" :size="24"><Loading /></el-icon>
        <span>{{ t('system.queueMonitor.loading') }}</span>
      </div>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.queue-monitor-page {
  display: flex;
  min-height: 0;
  height: 100%;
}
.queue-monitor-frame {
  flex: 1;
  width: 100%;
  min-height: 560px;
  border: 1px solid var(--el-border-color-light);
  background: #fff;
}
.queue-monitor-frame-shell {
  position: relative;
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 560px;
}
.queue-monitor-frame-loading {
  position: absolute;
  inset: 1px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 10px;
  color: var(--el-text-color-secondary);
  background: var(--el-bg-color);
}
.queue-monitor-state {
  display: grid;
  place-items: center;
  gap: 12px;
  width: 100%;
  min-height: 320px;
  color: var(--el-text-color-secondary);
  text-align: center;
}
</style>
