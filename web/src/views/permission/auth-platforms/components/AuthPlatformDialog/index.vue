<script setup lang="ts">
import { toRefs } from 'vue'
import { CircleHelp, RotateCcw } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { AppDialog } from '@/components/AppDialog'
import { YesNo } from '@/enums/yes-no'
import type { AuthPlatformForm } from './types'

const props = defineProps<{
  dialogMode: 'create' | 'edit'
  isEditing: boolean
  isBuiltinAdminEdit: boolean
  submitting: boolean
  formValid: boolean
}>()
const { dialogMode, isEditing, isBuiltinAdminEdit, submitting, formValid } = toRefs(props)
const visible = defineModel<boolean>({ required: true })
const form = defineModel<AuthPlatformForm>('form', { required: true })
const emit = defineEmits<{ save: []; 'restore-defaults': [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t(dialogMode === 'create' ? 'authPlatform.createTitle' : 'authPlatform.editTitle')"
    width="800px"
    :append-to-body="false"
  >
    <el-form
      label-position="top"
      class="auth-platform-form auth-platform-form-scroll"
      data-testid="auth-platform-form"
    >
      <div class="auth-platform-form-section">
        <h3>{{ t('authPlatform.form.basicSection') }}</h3>
        <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--basic">
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('authPlatform.code')">
              <el-input
                v-model="form.code"
                data-testid="auth-platform-code"
                :disabled="isEditing"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('authPlatform.name')">
              <el-input v-model="form.name" data-testid="auth-platform-name" />
            </el-form-item>
          </el-col>
        </el-row>
      </div>
      <div class="auth-platform-form-section">
        <div class="auth-platform-form-section__heading">
          <h3>{{ t('authPlatform.form.tokenSection') }}</h3>
          <el-button
            text
            type="primary"
            :icon="RotateCcw"
            data-testid="auth-platform-ttl-defaults"
            @click="emit('restore-defaults')"
            >{{ t('authPlatform.form.restoreTTLDefaults') }}</el-button
          >
        </div>
        <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--four">
          <el-col
            v-for="field in [
              {
                key: 'accessTTLSeconds',
                testId: 'auth-platform-access-ttl',
                label: 'authPlatform.accessTTL',
                help: 'authPlatform.form.accessTTLHelp',
                min: 60,
                max: 2_592_000,
              },
              {
                key: 'refreshTTLSeconds',
                testId: 'auth-platform-refresh-ttl',
                label: 'authPlatform.refreshTTL',
                help: 'authPlatform.form.refreshTTLHelp',
                min: 60,
                max: 31_536_000,
              },
              {
                key: 'sessionCacheTTLSeconds',
                testId: 'auth-platform-session-cache-ttl',
                label: 'authPlatform.sessionCacheTTL',
                help: 'authPlatform.form.sessionCacheTTLHelp',
                min: 60,
                max: 86_400,
              },
              {
                key: 'accessCacheTTLSeconds',
                testId: 'auth-platform-access-cache-ttl',
                label: 'authPlatform.accessCacheTTL',
                help: 'authPlatform.form.accessCacheTTLHelp',
                min: 60,
                max: 86_400,
              },
            ]"
            :key="field.key"
            :xs="24"
            :sm="12"
            :lg="6"
          >
            <el-form-item>
              <template #label>
                <span class="auth-platform-field-label auth-platform-field-label--nowrap"
                  >{{ t(field.label) }}
                  <el-tooltip :content="t(field.help)" placement="top">
                    <CircleHelp data-testid="auth-platform-ttl-help" aria-hidden="true" />
                  </el-tooltip>
                </span>
              </template>
              <el-input-number
                v-model="form[field.key]"
                :min="field.min"
                :max="field.max"
                class="auth-platform-number"
                :data-testid="field.testId"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </div>
      <div class="auth-platform-form-section">
        <h3>{{ t('authPlatform.form.policySection') }}</h3>
        <el-row
          :gutter="16"
          class="auth-platform-form-grid auth-platform-form-grid--four auth-platform-policy-grid auth-platform-policy-grid--three-up"
        >
          <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.bindDevice')">
              <el-switch
                v-model="form.bindDevice"
                :active-value="YesNo.Yes"
                :inactive-value="YesNo.No"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.bindIP')">
              <el-switch
                v-model="form.bindIP"
                :active-value="YesNo.Yes"
                :inactive-value="YesNo.No"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.allowRegister')">
              <el-switch
                v-model="form.allowRegister"
                :active-value="YesNo.Yes"
                :inactive-value="YesNo.No"
                :disabled="isBuiltinAdminEdit"
                data-testid="auth-platform-allow-register"
              />
              <span v-if="isBuiltinAdminEdit" class="auth-platform-form-help">{{
                t('authPlatform.adminRegistrationLocked')
              }}</span>
            </el-form-item>
          </el-col>
          <el-col v-if="dialogMode === 'create'" :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.isEnabled')">
              <el-switch
                v-model="form.isEnabled"
                :active-value="YesNo.Yes"
                :inactive-value="YesNo.No"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--session">
          <el-col :xs="24" :sm="12" :lg="8">
            <el-form-item :label="t('authPlatform.maxSessionsField')">
              <el-input-number
                v-model="form.maxSessions"
                :min="0"
                :max="100"
                class="auth-platform-number"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </div>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('authPlatform.cancel') }}</el-button>
      <el-button
        type="primary"
        :loading="submitting"
        :disabled="!formValid"
        @click="emit('save')"
        >{{ t('authPlatform.save') }}</el-button
      >
    </template>
  </AppDialog>
</template>

<style scoped src="./AuthPlatformDialog.css"></style>
