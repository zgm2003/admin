<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RefreshRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { getCaptcha } from '@/api/auth/login'
import type { CaptchaAnswer, CaptchaChallenge } from '@/api/auth/login'

const props = defineProps<{ modelValue: boolean; loading?: boolean }>()
defineOptions({ name: 'CaptchaDialog' })
const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'complete', value: { captchaId: string; captchaAnswer: CaptchaAnswer }): void
}>()
const { t } = useI18n()
const challenge = ref<CaptchaChallenge | null>(null)
const sliderX = ref(0)
const loadingChallenge = ref(false)
const verifying = ref(false)
const maxX = computed(() => Math.max(0, (challenge.value?.imageWidth ?? 300) - (challenge.value?.tileWidth ?? 48)))

watch(() => props.modelValue, (visible) => {
  if (visible) void refresh()
  else challenge.value = null
})

async function refresh(): Promise<void> {
  loadingChallenge.value = true
  try {
    challenge.value = await getCaptcha()
    sliderX.value = challenge.value.tileX
  } catch {
    challenge.value = null
    ElMessage.error(t('auth.captcha.loadFailed'))
  } finally {
    loadingChallenge.value = false
  }
}

function close(): void {
  if (verifying.value) return
  emit('update:modelValue', false)
}

function complete(): void {
  const current = challenge.value
  if (!current || loadingChallenge.value || props.loading || verifying.value) return
  verifying.value = true
  emit('complete', { captchaId: current.captchaId, captchaAnswer: { x: Math.round(sliderX.value), y: current.tileY } })
  verifying.value = false
}

function done(): void {
  verifying.value = false
}

defineExpose({ done, refresh })
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    width="360px"
    :title="t('auth.captcha.title')"
    append-to-body
    @update:model-value="emit('update:modelValue', $event)"
    @closed="close"
  >
    <div class="captcha-dialog" data-testid="captcha-dialog">
      <div v-if="challenge" class="captcha-image" :style="{ width: `${challenge.imageWidth}px`, height: `${challenge.imageHeight}px`, backgroundImage: `url(${challenge.masterImage})` }">
        <img class="captcha-tile" :src="challenge.tileImage" :alt="t('auth.captcha.tileAlt')" :style="{ width: `${challenge.tileWidth}px`, height: `${challenge.tileHeight}px`, left: `${sliderX}px`, top: `${challenge.tileY}px` }" />
      </div>
      <el-skeleton v-else :rows="5" animated />
      <el-slider v-model="sliderX" :min="0" :max="maxX" :disabled="loadingChallenge || verifying" aria-label="captcha slider" />
      <div class="captcha-actions">
        <el-button text :disabled="loadingChallenge || verifying" @click="refresh">
          <el-icon><RefreshRight /></el-icon>{{ t('auth.captcha.refresh') }}
        </el-button>
        <el-button type="primary" :loading="verifying" :disabled="!challenge || loadingChallenge" @click="complete">
          {{ t('auth.captcha.confirm') }}
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
.captcha-dialog { display: grid; gap: 14px; justify-items: center; }
.captcha-image { position: relative; overflow: hidden; max-width: 100%; background-size: 100% 100%; border-radius: 8px; }
.captcha-tile { position: absolute; pointer-events: none; }
.captcha-actions { display: flex; justify-content: space-between; width: 100%; }
</style>
