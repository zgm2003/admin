<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'
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
  <section class="brand-settings" :aria-label="t('setting.brandTitle')">
    <header class="brand-settings__header">
      <div class="brand-settings__intro">
        <span class="brand-settings__eyebrow">{{ t('setting.siteInfoTitle') }}</span>
        <h2>{{ t('setting.brandIdentity') }}</h2>
        <p>{{ t('setting.brandIdentityDescription') }}</p>
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
    <div v-else class="brand-settings__workspace">
      <el-form class="brand-settings__identity-card" label-position="top">
        <div class="brand-settings__card-heading">
          <h3>{{ t('setting.brandTitles') }}</h3>
          <p>{{ t('setting.brandTitlesDescription') }}</p>
        </div>
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
      </el-form>
      <aside class="brand-settings__avatar-card">
        <div class="brand-settings__card-heading">
          <h3>{{ t('setting.defaultAvatar') }}</h3>
          <p>{{ t('setting.defaultAvatarDescription') }}</p>
        </div>
        <el-form-item class="brand-settings__avatar">
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
      </aside>
    </div>
  </section>
</template>

<style scoped>
.brand-settings {
  width: min(1120px, 100%);
  margin: 0 auto;
}

.brand-settings__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 32px;
  padding: 4px 0 20px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.brand-settings__intro {
  min-width: 0;
}

.brand-settings__eyebrow {
  display: block;
  margin-bottom: 8px;
  color: var(--el-color-primary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.brand-settings__intro h2,
.brand-settings__card-heading h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-weight: 600;
}

.brand-settings__intro h2 {
  font-size: 20px;
  line-height: 1.35;
}

.brand-settings__intro p,
.brand-settings__card-heading p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.brand-settings__workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 320px);
  gap: 20px;
  margin-top: 24px;
}

.brand-settings__identity-card,
.brand-settings__avatar-card {
  min-width: 0;
  padding: 24px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-bg-color);
  box-shadow: 0 8px 24px rgb(15 23 42 / 4%);
}

.brand-settings__card-heading {
  margin-bottom: 22px;
}

.brand-settings__card-heading h3 {
  font-size: 15px;
  line-height: 1.4;
}

.brand-settings__titles {
  display: grid;
  gap: 4px;
}

.brand-settings__avatar-card {
  display: flex;
  flex-direction: column;
}

.brand-settings__avatar {
  display: flex;
  justify-content: center;
  margin: auto 0 0;
  padding-top: 20px;
}

.brand-settings__avatar :deep(.el-form-item__content) {
  justify-content: center;
}

.brand-settings__header .el-button {
  flex: 0 0 auto;
}

@media (max-width: 760px) {
  .brand-settings__header {
    align-items: stretch;
    flex-direction: column;
    gap: 16px;
  }

  .brand-settings__header .el-button {
    width: 100%;
  }

  .brand-settings__workspace {
    grid-template-columns: 1fr;
  }

  .brand-settings__avatar {
    margin-top: 0;
  }
}
</style>
