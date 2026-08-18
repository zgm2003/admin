<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { User } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'

import { getCurrentUser, login } from '../../../api/auth'
import { useAuthStore } from '../../../store/auth'
import { ApiError } from '../../../types/http'

interface LoginForm {
  username: string
  password: string
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const formReference = ref<FormInstance>()
const form = reactive<LoginForm>({ username: '', password: '' })
const pending = ref(false)
const submitError = ref('')
const bootstrapError = computed(() => auth.status === 'error' ? auth.errorMessage : '')
const rules: FormRules<LoginForm> = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function submit(): Promise<void> {
  if (pending.value || formReference.value === undefined) return
  if (form.username.trim() === '' || form.password === '') return
  const valid = await formReference.value.validate().catch(() => false)
  if (!valid) return
  pending.value = true
  submitError.value = ''
  try {
    const credential = await login({ username: form.username, password: form.password })
    auth.setCredential(credential)
    const currentUser = await getCurrentUser()
    auth.setAuthenticated(currentUser)
    await router.replace(safeRedirect(route.query.redirect))
  } catch (error: unknown) {
    auth.setAnonymous()
    submitError.value = error instanceof ApiError && error.code === 10002
      ? '用户名或密码错误'
      : error instanceof Error && error.message !== '' ? error.message : '登录失败'
  } finally {
    pending.value = false
  }
}

function safeRedirect(value: unknown): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/dashboard'
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-panel" aria-labelledby="login-title">
      <div class="auth-signature" aria-hidden="true"></div>
      <el-icon class="auth-icon"><User /></el-icon>
      <h1 id="login-title">登录管理台</h1>
      <p class="auth-caption">使用用户名继续</p>

      <p v-if="bootstrapError" class="auth-error" data-testid="bootstrap-error">{{ bootstrapError }}</p>
      <p v-if="submitError" class="auth-error" data-testid="login-error">{{ submitError }}</p>

      <el-form ref="formReference" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" data-testid="login-username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" data-testid="login-password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-button data-testid="login-submit" type="primary" native-type="submit" :loading="pending" :disabled="pending">
          登录
        </el-button>
      </el-form>

      <RouterLink class="auth-link" to="/register">注册新账号</RouterLink>
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
  padding: 32px 0;
}

.auth-signature {
  width: 44px;
  height: 3px;
  margin-bottom: 28px;
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
  margin: 8px 0 28px;
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
