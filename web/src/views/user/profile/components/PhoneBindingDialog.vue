<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { bindPhone, sendPhoneCode } from '@/api/user/phone'
import { AppDialog } from '@/components/AppDialog'

const props = defineProps<{ currentPhone: string | null }>()
const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ updated: [phone: string] }>()
const { t } = useI18n()
const form = reactive({ nextPhone: '', currentCode: '', nextCode: '' })
const currentChallengeID = ref('')
const nextChallengeID = ref('')
const currentResendSeconds = ref(0)
const nextResendSeconds = ref(0)
const sendingCurrent = ref(false)
const sendingNext = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const timers = new Set<number>()
const changing = computed(() => props.currentPhone !== null)
const canSubmit = computed(
  () =>
    nextChallengeID.value !== '' &&
    /^\d{6}$/.test(form.nextCode) &&
    (!changing.value ||
      (currentChallengeID.value !== '' && /^\d{6}$/.test(form.currentCode))),
)

function startCountdown(value: typeof currentResendSeconds, seconds: number): void {
  value.value = seconds
  if (seconds <= 0) return
  const timer = window.setInterval(() => {
    value.value = Math.max(0, value.value - 1)
    if (value.value === 0) {
      window.clearInterval(timer)
      timers.delete(timer)
    }
  }, 1000)
  timers.add(timer)
}

async function sendCurrent(): Promise<void> {
  if (sendingCurrent.value || currentResendSeconds.value > 0) return
  sendingCurrent.value = true
  errorMessage.value = ''
  try {
    const result = await sendPhoneCode({ target: 'current' })
    currentChallengeID.value = result.challengeId
    startCountdown(currentResendSeconds, result.resendAfterSeconds)
  } catch (error: unknown) {
    errorMessage.value = error instanceof Error ? error.message : t('user.identity.sendFailed')
  } finally {
    sendingCurrent.value = false
  }
}

async function sendNext(): Promise<void> {
  const phone = form.nextPhone.trim()
  if (sendingNext.value || nextResendSeconds.value > 0 || phone === '') return
  sendingNext.value = true
  errorMessage.value = ''
  try {
    const result = await sendPhoneCode({ target: 'next', phone })
    nextChallengeID.value = result.challengeId
    startCountdown(nextResendSeconds, result.resendAfterSeconds)
  } catch (error: unknown) {
    errorMessage.value = error instanceof Error ? error.message : t('user.identity.sendFailed')
  } finally {
    sendingNext.value = false
  }
}

async function submit(): Promise<void> {
  if (!canSubmit.value || saving.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    const input = {
      nextPhone: form.nextPhone.trim(),
      nextChallengeId: nextChallengeID.value,
      nextCode: form.nextCode,
      ...(changing.value
        ? {
            currentChallengeId: currentChallengeID.value,
            currentCode: form.currentCode,
          }
        : {}),
    }
    const result = await bindPhone(input)
    emit('updated', result.phone)
    visible.value = false
    ElMessage.success(t('user.identity.phoneSaved'))
  } catch (error: unknown) {
    errorMessage.value = error instanceof Error ? error.message : t('user.identity.saveFailed')
  } finally {
    saving.value = false
  }
}

function reset(): void {
  form.nextPhone = ''
  form.currentCode = ''
  form.nextCode = ''
  currentChallengeID.value = ''
  nextChallengeID.value = ''
  currentResendSeconds.value = 0
  nextResendSeconds.value = 0
  errorMessage.value = ''
  for (const timer of timers) window.clearInterval(timer)
  timers.clear()
}

watch(visible, (value) => {
  if (value) reset()
})
onBeforeUnmount(reset)
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t(changing ? 'user.identity.changePhone' : 'user.identity.bindPhone')"
    width="min(520px, 94vw)"
    append-to-body
  >
    <el-alert
      v-if="errorMessage"
      :title="errorMessage"
      type="error"
      :closable="false"
      show-icon
    />
    <el-form label-position="top" @submit.prevent="submit">
      <template v-if="changing">
        <el-form-item :label="t('user.identity.currentPhone')">
          <el-input :model-value="currentPhone ?? ''" disabled />
        </el-form-item>
        <el-form-item :label="t('user.identity.currentCode')">
          <div class="identity-proof-row">
            <el-input
              v-model="form.currentCode"
              data-testid="profile-phone-current-code"
              maxlength="6"
              inputmode="numeric"
              :placeholder="t('user.identity.codePlaceholder')"
            />
            <el-button
              data-testid="profile-phone-current-send"
              :loading="sendingCurrent"
              :disabled="currentResendSeconds > 0"
              @click="sendCurrent"
            >
              {{
                currentResendSeconds > 0
                  ? t('user.identity.resendIn', { seconds: currentResendSeconds })
                  : t('user.identity.sendCode')
              }}
            </el-button>
          </div>
        </el-form-item>
      </template>
      <el-form-item :label="t('user.identity.nextPhone')">
        <el-input
          v-model="form.nextPhone"
          data-testid="profile-phone-next"
          type="tel"
          inputmode="tel"
          autocomplete="tel"
          :placeholder="t('user.identity.phonePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('user.identity.nextCode')">
        <div class="identity-proof-row">
          <el-input
            v-model="form.nextCode"
            data-testid="profile-phone-next-code"
            maxlength="6"
            inputmode="numeric"
            :placeholder="t('user.identity.codePlaceholder')"
          />
          <el-button
            data-testid="profile-phone-next-send"
            :loading="sendingNext"
            :disabled="nextResendSeconds > 0 || form.nextPhone.trim() === ''"
            @click="sendNext"
          >
            {{
              nextResendSeconds > 0
                ? t('user.identity.resendIn', { seconds: nextResendSeconds })
                : t('user.identity.sendCode')
            }}
          </el-button>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('user.cancel') }}</el-button>
      <el-button
        data-testid="profile-phone-submit"
        type="primary"
        :loading="saving"
        :disabled="!canSubmit"
        @click="submit"
      >
        {{ t('user.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped>
.identity-proof-row {
  display: grid;
  width: 100%;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

@media (max-width: 480px) {
  .identity-proof-row {
    grid-template-columns: 1fr;
  }
}
</style>
