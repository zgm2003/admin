<script setup lang="ts">
import { reactive, ref } from 'vue'
import { EditPen } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormItemRule, type FormRules } from 'element-plus'
import { useRouter } from 'vue-router'

import { register } from '../../../api/auth'

interface RegisterForm {
  username: string
  email: string
  password: string
  confirmPassword: string
}

const router = useRouter()
const formReference = ref<FormInstance>()
const form = reactive<RegisterForm>({ username: '', email: '', password: '', confirmPassword: '' })
const pending = ref(false)
const submitError = ref('')
const confirmPasswordValidator: FormItemRule['validator'] = (_rule, value, callback) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
    return
  }
  callback()
}
const rules: FormRules<RegisterForm> = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效邮箱', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 个字符', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    { validator: confirmPasswordValidator, trigger: 'blur' },
  ],
}

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
    ElMessage.success('注册成功')
    await router.replace('/login')
  } catch (error: unknown) {
    submitError.value = error instanceof Error && error.message !== '' ? error.message : '注册失败'
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-panel" aria-labelledby="register-title">
      <div class="auth-signature" aria-hidden="true"></div>
      <el-icon class="auth-icon"><EditPen /></el-icon>
      <h1 id="register-title">注册账号</h1>
      <p class="auth-caption">创建管理台登录身份</p>

      <p v-if="submitError" class="auth-error" data-testid="register-error">{{ submitError }}</p>

      <el-form ref="formReference" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" data-testid="register-username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" data-testid="register-email" autocomplete="email" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" data-testid="register-password" type="password" autocomplete="new-password" show-password />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" data-testid="register-confirm-password" type="password" autocomplete="new-password" show-password />
        </el-form-item>
        <el-button data-testid="register-submit" type="primary" native-type="submit" :loading="pending" :disabled="pending">
          注册
        </el-button>
      </el-form>

      <RouterLink class="auth-link" to="/login">返回登录</RouterLink>
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
