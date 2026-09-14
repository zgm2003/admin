<script setup lang="ts">
import { onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { grantQueueMonitor, QUEUE_MONITOR_UI_URL } from '@/api/system/queueMonitor'
import { usePermissionStore } from '@/store/permission'

const { t } = useI18n()
const access = usePermissionStore()
const loading = ref(false)
const loaded = ref(false)
const failed = ref(false)
const frameFailed = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

function clearRenewal(): void {
  if (timer !== undefined) { clearInterval(timer); timer = undefined }
}
function startRenewal(): void {
  clearRenewal()
  timer = setInterval(() => { void grant() }, 45_000)
}
async function grant(): Promise<void> {
  if (!access.hasPermission('system:queueMonitor:list')) return
  loading.value = true
  failed.value = false
  try { await grantQueueMonitor(); loaded.value = true; frameFailed.value = false; startRenewal() }
  catch { loaded.value = false; failed.value = true; clearRenewal() }
  finally { loading.value = false }
}
function onFrameError(): void { frameFailed.value = true }
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
      <el-button type="primary" :loading="loading" @click="grant">{{ t('system.queueMonitor.retry') }}</el-button>
    </div>
    <div v-else-if="!loaded" class="queue-monitor-state">{{ t('system.queueMonitor.loading') }}</div>
    <div v-else-if="frameFailed" class="queue-monitor-state">
      <p>{{ t('system.queueMonitor.loadFailed') }}</p>
      <el-button type="primary" :loading="loading" @click="grant">{{ t('system.queueMonitor.retry') }}</el-button>
    </div>
    <iframe v-else class="queue-monitor-frame" :src="QUEUE_MONITOR_UI_URL" :title="t('system.queueMonitor.title')" @error="onFrameError" />
  </AppPage>
</template>

<style scoped>
.queue-monitor-page { display: flex; min-height: 0; height: 100%; }
.queue-monitor-frame { flex: 1; width: 100%; min-height: 560px; border: 1px solid var(--el-border-color-light); background: #fff; }
.queue-monitor-state { display: grid; place-items: center; gap: 12px; width: 100%; min-height: 320px; color: var(--el-text-color-secondary); text-align: center; }
</style>
