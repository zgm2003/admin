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
    <header v-if="canUpdate" class="brand-settings__header">
      <el-button
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
  width: min(920px, 100%);
}

.brand-settings__header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.brand-settings__form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 160px;
  gap: 32px;
}

.brand-settings__titles {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
}

.brand-settings__avatar {
  margin-bottom: 0;
}

@media (max-width: 760px) {
  .brand-settings__form {
    display: block;
  }

  .brand-settings__header .el-button {
    width: 100%;
  }

  .brand-settings__titles {
    gap: 0;
  }

  .brand-settings__avatar {
    margin-top: 4px;
  }
}
</style>
