<script setup lang="ts">
import { Check, Picture } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { BrandSettings } from '@/api/system/setting'
import UpMedia from '@/components/UpMedia/index.vue'

defineOptions({ name: 'BrandSettingsPanel' })

defineProps<{
  loading: boolean
  saving: boolean
  error: string
  canUpdate: boolean
  canUpload: boolean
}>()
const form = defineModel<BrandSettings>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <section class="brand-settings" aria-labelledby="brand-settings-title">
    <header class="brand-settings__header">
      <div>
        <div class="brand-settings__eyebrow">
          <el-icon><Picture /></el-icon>
          <span>{{ t('setting.brandIdentity') }}</span>
        </div>
        <h2 id="brand-settings-title">{{ t('setting.brandTitle') }}</h2>
      </div>
      <el-button
        v-if="canUpdate"
        data-testid="brand-save"
        type="primary"
        :icon="Check"
        :loading="saving"
        @click="emit('save')"
      >
        {{ t('setting.saveBrand') }}
      </el-button>
    </header>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />

    <el-skeleton v-if="loading" :rows="3" animated />
    <el-form v-else class="brand-settings__form" label-position="top">
      <div class="brand-settings__titles">
        <el-form-item :label="t('setting.brandTitleZhCN')" required>
          <el-input
            v-model="form.titleZhCN"
            data-testid="brand-title-zh-cn"
            :maxlength="128"
            :disabled="!canUpdate"
          />
        </el-form-item>
        <el-form-item :label="t('setting.brandTitleEnUS')" required>
          <el-input
            v-model="form.titleEnUS"
            data-testid="brand-title-en-us"
            :maxlength="128"
            :disabled="!canUpdate"
          />
        </el-form-item>
      </div>
      <el-form-item class="brand-settings__avatar" :label="t('setting.defaultAvatar')">
        <UpMedia
          v-model="form.defaultAvatar"
          rule-code="avatar"
          :multiple="false"
          accept="image/*"
          variant="avatar"
          width="104px"
          :disabled="!canUpdate || !canUpload"
        />
      </el-form-item>
    </el-form>
  </section>
</template>

<style scoped>
.brand-settings {
  padding: 2px 0 22px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.brand-settings__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.brand-settings__eyebrow {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 5px;
  color: var(--el-color-primary);
  font-size: 12px;
  font-weight: 600;
}

.brand-settings h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 18px;
  line-height: 1.4;
}

.brand-settings__form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 180px;
  gap: 28px;
}

.brand-settings__titles {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.brand-settings__avatar {
  margin-bottom: 0;
}

@media (max-width: 760px) {
  .brand-settings__header,
  .brand-settings__form {
    display: block;
  }

  .brand-settings__header .el-button {
    width: 100%;
    margin-top: 14px;
  }

  .brand-settings__titles {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .brand-settings__avatar {
    margin-top: 4px;
  }
}
</style>
