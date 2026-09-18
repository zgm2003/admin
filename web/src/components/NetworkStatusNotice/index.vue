<script setup lang="ts">
import { RefreshCw, WifiOff } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useNetworkStatus } from '@/composables/useNetworkStatus'

const { locale, t } = useI18n()
const { isOffline, lastOfflineAt, refreshPage } = useNetworkStatus()

const offlineTime = computed(() => {
  if (lastOfflineAt.value === null) return ''
  return new Intl.DateTimeFormat(locale.value, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(lastOfflineAt.value)
})
</script>

<template>
  <Transition name="network-status">
    <aside
      v-if="isOffline"
      class="network-status"
      role="status"
      aria-live="assertive"
      data-testid="network-status-notice"
    >
      <WifiOff class="network-status__icon" :size="20" :stroke-width="2" aria-hidden="true" />
      <div class="network-status__content">
        <strong>{{ t('network.offline.title') }}</strong>
        <span>
          {{ t('network.offline.message') }}
          <small v-if="offlineTime">{{ t('network.offline.since', { time: offlineTime }) }}</small>
        </span>
      </div>
      <button
        class="network-status__refresh"
        type="button"
        :aria-label="t('network.offline.refresh')"
        :title="t('network.offline.refresh')"
        data-testid="network-status-refresh"
        @click="refreshPage"
      >
        <RefreshCw :size="17" :stroke-width="2" aria-hidden="true" />
      </button>
    </aside>
  </Transition>
</template>

<style scoped>
.network-status {
  position: fixed;
  z-index: 4000;
  top: 12px;
  left: 50%;
  display: grid;
  grid-template-columns: 20px minmax(0, 1fr) 32px;
  gap: 10px;
  align-items: center;
  width: min(440px, calc(100vw - 32px));
  min-height: 56px;
  padding: 8px 10px 8px 12px;
  color: var(--el-text-color-primary);
  border: 1px solid var(--el-color-warning-light-5);
  border-radius: 8px;
  background: color-mix(in srgb, var(--el-bg-color-overlay) 92%, var(--el-color-warning-light-9));
  box-shadow: var(--admin-shadow-md);
  transform: translateX(-50%);
}

.network-status__icon {
  color: var(--el-color-warning-dark-2);
}

.network-status__content {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.network-status__content strong {
  font-size: 13px;
  font-weight: 650;
  line-height: 1.35;
}

.network-status__content span {
  color: var(--el-text-color-regular);
  font-size: 12px;
  line-height: 1.45;
}

.network-status__content small {
  margin-left: 6px;
  color: var(--el-text-color-secondary);
  font-size: inherit;
}

.network-status__refresh {
  display: grid;
  width: 32px;
  height: 32px;
  padding: 0;
  place-items: center;
  color: var(--el-text-color-regular);
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  background: var(--el-fill-color-blank);
  cursor: pointer;
}

.network-status__refresh:hover {
  color: var(--el-color-warning-dark-2);
  border-color: var(--el-color-warning-light-3);
  background: var(--el-color-warning-light-9);
}

.network-status__refresh:focus-visible {
  outline: 2px solid var(--el-color-warning);
  outline-offset: 2px;
}

.network-status-enter-active,
.network-status-leave-active {
  transition:
    opacity 160ms ease,
    transform 180ms ease;
}

.network-status-enter-from,
.network-status-leave-to {
  opacity: 0;
  transform: translate(-50%, -8px);
}

@media (max-width: 540px) {
  .network-status {
    top: 8px;
    width: calc(100vw - 16px);
  }

  .network-status__content small {
    display: block;
    margin-left: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .network-status-enter-active,
  .network-status-leave-active {
    transition: none;
  }
}
</style>
