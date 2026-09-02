<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import { AppDialog } from '@/components/AppDialog'
import type { ConfigForm } from '@/views/cloud/storage-object/components/types'

const props = defineProps<{
  editing: boolean
  rules: FormRules<ConfigForm>
  urlErrors: { endpoint: string; bucketDomain: string }
  regions: readonly { value: string; label: string }[]
  validateUrlField: (field: 'endpoint' | 'bucketDomain') => void
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<ConfigForm>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
const formRef = ref<FormInstance>()
defineExpose({ validate: () => formRef.value?.validate() })
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="props.editing ? t('storage.editConfig') : t('storage.addConfig')"
    width="min(720px, 94vw)"
    :append-to-body="false"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="props.rules"
      label-position="top"
      data-testid="storage-config-form"
    >
      <el-row :gutter="16">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.name')" prop="name">
            <el-input
              v-model="form.name"
              data-testid="storage-config-name"
              :placeholder="t('storage.namePlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.appId')" prop="appId">
            <el-input
              v-model="form.appId"
              data-testid="storage-config-app-id"
              :placeholder="t('storage.appIdPlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.secretId')" prop="secretId">
            <el-input
              v-model="form.secretId"
              data-testid="storage-config-secret-id"
              type="password"
              show-password
              :placeholder="
                props.editing
                  ? t('storage.secretKeepPlaceholder')
                  : t('storage.secretIdPlaceholder')
              "
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.secretKey')" prop="secretKey">
            <el-input
              v-model="form.secretKey"
              data-testid="storage-config-secret-key"
              type="password"
              show-password
              :placeholder="
                props.editing
                  ? t('storage.secretKeepPlaceholder')
                  : t('storage.secretKeyPlaceholder')
              "
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.bucket')" prop="bucket">
            <el-input
              v-model="form.bucket"
              data-testid="storage-config-bucket"
              :placeholder="t('storage.bucketPlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('storage.region')" prop="region">
            <el-select
              v-model="form.region"
              data-testid="storage-config-region"
              :placeholder="t('storage.regionPlaceholder')"
            >
              <el-option
                v-for="item in props.regions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.endpoint')" :error="props.urlErrors.endpoint"
            ><el-input
              v-model="form.endpoint"
              data-testid="storage-config-endpoint"
              :placeholder="t('storage.endpointPlaceholder')"
              @blur="props.validateUrlField('endpoint')" /></el-form-item
        ></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.bucketDomain')" :error="props.urlErrors.bucketDomain"
            ><el-input
              v-model="form.bucketDomain"
              data-testid="storage-config-domain"
              :placeholder="t('storage.domainPlaceholder')"
              @blur="props.validateUrlField('bucketDomain')" /></el-form-item
        ></el-col>
        <el-col :xs="24"
          ><el-form-item :label="t('storage.remark')"
            ><el-input
              v-model="form.remark"
              type="textarea"
              :rows="2"
              :placeholder="t('storage.remarkPlaceholder')" /></el-form-item
        ></el-col>
      </el-row>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('storage.cancel') }}</el-button>
      <el-button type="primary" @click="emit('save')">{{ t('storage.save') }}</el-button>
    </template>
  </AppDialog>
</template>
