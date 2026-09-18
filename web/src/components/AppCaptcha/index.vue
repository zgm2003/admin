<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Slide as GoCaptchaSlide } from 'go-captcha-vue'
import 'go-captcha-vue/dist/style.css'
import { RefreshCw } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { getCaptcha } from '@/api/auth/login'
import type { CaptchaAnswer, CaptchaChallenge } from '@/api/auth/login'

interface SlideConfig {
  width: number
  height: number
  thumbWidth: number
  thumbHeight: number
  verticalPadding: number
  horizontalPadding: number
  showTheme: boolean
  title: string
  iconSize: number
  scope: boolean
}

interface SlideData {
  thumbX: number
  thumbY: number
  thumbWidth: number
  thumbHeight: number
  image: string
  thumb: string
}

interface SlidePoint {
  x: number
  y: number
}

interface SlideEvents {
  move: (x: number, y: number) => void
  refresh: () => void
  close: () => void
  confirm: (point: SlidePoint) => void
}

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    loading?: boolean
  }>(),
  { loading: false },
)

defineOptions({ name: 'AppCaptcha' })

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'complete', value: { captchaId: string; captchaAnswer: CaptchaAnswer }): void
}>()

const { t } = useI18n()
const overlay = ref<HTMLElement | null>(null)
const challenge = ref<CaptchaChallenge | null>(null)
const sliderX = ref(0)
const loadingChallenge = ref(false)
const loadFailed = ref(false)
let requestGeneration = 0

const slideConfig = computed<SlideConfig>(() => ({
  width: challenge.value?.imageWidth ?? 300,
  height: challenge.value?.imageHeight ?? 220,
  thumbWidth: challenge.value?.tileWidth ?? 48,
  thumbHeight: challenge.value?.tileHeight ?? 48,
  verticalPadding: 12,
  horizontalPadding: 16,
  showTheme: true,
  title: t('auth.captcha.instruction'),
  iconSize: 22,
  scope: true,
}))

const slideData = computed<SlideData>(() => {
  const current = challenge.value
  if (current === null) {
    return {
      thumbX: 0,
      thumbY: 0,
      thumbWidth: 0,
      thumbHeight: 0,
      image: '',
      thumb: '',
    }
  }
  return {
    thumbX: current.tileX,
    thumbY: current.tileY,
    thumbWidth: current.tileWidth,
    thumbHeight: current.tileHeight,
    image: current.masterImage,
    thumb: current.tileImage,
  }
})

const slideEvents = computed<SlideEvents>(() => ({
  move: (x: number) => {
    if (!isBusy()) sliderX.value = Math.round(x)
  },
  refresh: () => {
    if (!isBusy()) void refresh()
  },
  close,
  confirm: (point: SlidePoint) => complete(point),
}))

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      void refresh()
      void nextTick(() => overlay.value?.focus())
      return
    }
    requestGeneration += 1
    challenge.value = null
    sliderX.value = 0
    loadingChallenge.value = false
    loadFailed.value = false
  },
  { immediate: true },
)

async function refresh(): Promise<void> {
  const generation = ++requestGeneration
  loadingChallenge.value = true
  loadFailed.value = false
  challenge.value = null
  sliderX.value = 0
  try {
    const nextChallenge = await getCaptcha()
    if (generation !== requestGeneration || !props.modelValue) return
    challenge.value = nextChallenge
    sliderX.value = nextChallenge.tileX
  } catch {
    if (generation !== requestGeneration || !props.modelValue) return
    loadFailed.value = true
  } finally {
    if (generation === requestGeneration) loadingChallenge.value = false
  }
}

function isBusy(): boolean {
  return loadingChallenge.value || props.loading
}

function close(): void {
  if (isBusy()) return
  emit('update:modelValue', false)
}

function complete(point: SlidePoint): void {
  const current = challenge.value
  if (current === null || isBusy()) return
  sliderX.value = Math.round(point.x)
  emit('complete', {
    captchaId: current.captchaId,
    captchaAnswer: { x: sliderX.value, y: current.tileY },
  })
}
</script>

<template>
  <Teleport to="body">
    <Transition name="app-captcha-fade">
      <div
        v-if="modelValue"
        ref="overlay"
        class="app-captcha-overlay"
        data-testid="app-captcha-overlay"
        role="dialog"
        aria-modal="true"
        :aria-label="t('auth.captcha.title')"
        tabindex="-1"
        @click.self="close"
        @keydown.esc="close"
      >
        <div class="app-captcha-panel" :aria-busy="isBusy()">
          <GoCaptchaSlide
            v-if="challenge"
            :config="slideConfig"
            :data="slideData"
            :events="slideEvents"
          />
          <div v-else-if="loadingChallenge" class="app-captcha-loading" role="status">
            <span class="app-captcha-loading__spinner" aria-hidden="true" />
            <span>{{ t('auth.captcha.loading') }}</span>
          </div>
          <div v-else-if="loadFailed" class="app-captcha-loading" role="alert">
            <span>{{ t('auth.captcha.loadFailed') }}</span>
            <button
              class="app-captcha-retry"
              type="button"
              data-testid="app-captcha-retry"
              @click="refresh"
            >
              <RefreshCw :size="15" aria-hidden="true" />
              <span>{{ t('auth.captcha.refresh') }}</span>
            </button>
          </div>
          <div v-if="loading" class="app-captcha-pending" aria-hidden="true">
            <span class="app-captcha-loading__spinner" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.app-captcha-overlay {
  position: fixed;
  inset: 0;
  z-index: 4000;
  display: grid;
  padding: 16px;
  place-items: center;
  background: rgb(15 23 42 / 52%);
  backdrop-filter: blur(2px);
  outline: none;
}

.app-captcha-panel {
  --go-captcha-theme-text-color: var(--el-text-color-primary);
  --go-captcha-theme-bg-color: var(--el-bg-color-overlay);
  --go-captcha-theme-btn-bg-color: var(--el-color-primary);
  --go-captcha-theme-btn-border-color: var(--el-color-primary);
  --go-captcha-theme-btn-disabled-color: var(--el-color-primary-light-3);
  --go-captcha-theme-active-color: var(--el-color-primary);
  --go-captcha-theme-border-color: var(--el-border-color-light);
  --go-captcha-theme-icon-color: var(--el-text-color-regular);
  --go-captcha-theme-drag-bar-color: var(--el-fill-color-dark);
  --go-captcha-theme-drag-bg-color: var(--el-color-primary);
  --go-captcha-theme-loading-icon-color: var(--el-color-primary);
  position: relative;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border-radius: 8px;
  box-shadow: var(--admin-shadow-lg);
}

.app-captcha-panel :deep(.go-captcha.gc-theme) {
  box-shadow: none;
}

.app-captcha-panel :deep(.gc-header) {
  font-weight: 600;
}

.app-captcha-loading {
  display: grid;
  width: 334px;
  min-height: 330px;
  place-content: center;
  justify-items: center;
  gap: 12px;
  color: var(--el-text-color-secondary);
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-light);
  font-size: 13px;
}

.app-captcha-loading__spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--el-border-color);
  border-top-color: var(--el-color-primary);
  border-radius: 50%;
  animation: app-captcha-spin 700ms linear infinite;
}

.app-captcha-retry {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 14px;
  gap: 7px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border: 1px solid var(--el-color-primary-light-7);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.app-captcha-retry:hover {
  background: var(--el-color-primary-light-8);
}

.app-captcha-retry:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}

.app-captcha-pending {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, var(--el-bg-color-overlay) 68%, transparent);
}

.app-captcha-fade-enter-active,
.app-captcha-fade-leave-active {
  transition: opacity 160ms ease;
}

.app-captcha-fade-enter-from,
.app-captcha-fade-leave-to {
  opacity: 0;
}

@keyframes app-captcha-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 380px) {
  .app-captcha-overlay {
    padding: 12px;
  }

  .app-captcha-panel {
    max-width: calc(100vw - 24px);
    transform: scale(0.92);
  }
}

@media (max-width: 340px) {
  .app-captcha-panel {
    transform: scale(0.86);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-captcha-loading__spinner {
    animation-duration: 1.8s;
  }
}
</style>
