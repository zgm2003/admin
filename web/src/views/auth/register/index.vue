<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { EditPen } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormItemRule, type FormRules } from 'element-plus'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { getAuthPolicy, register } from '../../../api/auth'
import { YesNo } from '../../../enums/yes-no'

interface RegisterForm {
  username: string
  email: string
  password: string
  confirmPassword: string
}

const router = useRouter()
const { t } = useI18n()
const formReference = ref<FormInstance>()
const form = reactive<RegisterForm>({ username: '', email: '', password: '', confirmPassword: '' })
const pending = ref(false)
const submitError = ref('')
const policyAllowed = ref(false)
const policyLoaded = ref(false)
const confirmPasswordValidator: FormItemRule['validator'] = (_rule, value, callback) => {
  if (value !== form.password) {
    callback(new Error(t('auth.register.passwordMismatch')))
    return
  }
  callback()
}
const rules: FormRules<RegisterForm> = {
  username: [{ required: true, message: t('auth.register.usernameRequired'), trigger: 'blur' }],
  email: [
    { required: true, message: t('auth.register.emailRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.register.emailInvalid'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.register.passwordRequired'), trigger: 'blur' },
    { min: 8, message: t('auth.register.passwordMin'), trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: t('auth.register.confirmPasswordRequired'), trigger: 'blur' },
    { validator: confirmPasswordValidator, trigger: 'blur' },
  ],
}

onMounted(async () => {
  try {
    const policy = await getAuthPolicy()
    if (policy.allowRegister === YesNo.No) {
      await router.replace('/login')
      return
    }
    policyAllowed.value = true
  } catch (error: unknown) {
    submitError.value = error instanceof Error && error.message !== ''
      ? error.message
      : t('auth.register.policyFailed')
  } finally {
    policyLoaded.value = true
  }
})

async function submit(): Promise<void> {
  if (pending.value || formReference.value === undefined) return
  if (form.username.trim() === '' || form.email.trim() === '' || form.password === '' || form.confirmPassword === '' || form.password !== form.confirmPassword) return
  const valid = await formReference.value.validate().catch(() => false)
  if (!valid) return
  pending.value = true
  submitError.value = ''
  try {
    await register({
      username: form.username,
      email: form.email,
      password: form.password,
      confirmPassword: form.confirmPassword,
    })
    ElMessage.success(t('auth.register.success'))
    await router.replace('/login')
  } catch (error: unknown) {
    submitError.value = error instanceof Error && error.message !== '' ? error.message : t('auth.register.failed')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <p v-if="submitError" class="auth-error" data-testid="register-error">{{ submitError }}</p>
    <section v-if="policyLoaded && policyAllowed" class="auth-panel" aria-labelledby="register-title">
      <div class="auth-signature" aria-hidden="true"></div>
      <el-icon class="auth-icon"><EditPen /></el-icon>
      <h1 id="register-title">{{ t('auth.register.title') }}</h1>
      <p class="auth-caption">{{ t('auth.register.caption') }}</p>

      <el-form ref="formReference" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item :label="t('auth.register.username')" prop="username">
          <el-input v-model="form.username" data-testid="register-username" autocomplete="username" />
        </el-form-item>
        <el-form-item :label="t('auth.register.email')" prop="email">
          <el-input v-model="form.email" data-testid="register-email" autocomplete="email" />
        </el-form-item>
        <el-form-item :label="t('auth.register.password')" prop="password">
          <el-input v-model="form.password" data-testid="register-password" type="password" autocomplete="new-password" show-password />
        </el-form-item>
        <el-form-item :label="t('auth.register.confirmPassword')" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" data-testid="register-confirm-password" type="password" autocomplete="new-password" show-password />
        </el-form-item>
        <el-button data-testid="register-submit" type="primary" native-type="submit" :loading="pending" :disabled="pending">
          {{ t('auth.register.submit') }}
        </el-button>
      </el-form>

      <RouterLink class="auth-link" to="/login">{{ t('auth.register.backToLogin') }}</RouterLink>
    </section>
  </main>
</template>

<style scoped>
.auth-page {
  display: grid;
  min-height: 100vh;
  padding: 32px 20px;
  place-items: center;
  background: #f4f6f7;
}

.auth-panel {
  position: relative;
  width: min(380px, 100%);
  padding: 24px 0;
}

.auth-signature {
  width: 44px;
  height: 3px;
  margin-bottom: 24px;
  background: #16756f;
}

.auth-icon {
  color: #16756f;
  font-size: 24px;
}

h1 {
  margin: 12px 0 0;
  color: #18212a;
  font-size: 24px;
  font-weight: 700;
}

.auth-caption {
  margin: 8px 0 24px;
  color: #71808d;
  font-size: 14px;
}

.el-button {
  width: 100%;
  margin-top: 4px;
}

.auth-link {
  display: inline-block;
  margin-top: 22px;
  color: #16756f;
  font-size: 14px;
  text-decoration: none;
}

.auth-error {
  margin: 0 0 16px;
  padding: 10px 12px;
  color: #a92f2f;
  background: #fff1f1;
  border-left: 3px solid #c33c3c;
  font-size: 13px;
}
</style>
