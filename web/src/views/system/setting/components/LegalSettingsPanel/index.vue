<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { LegalDocumentKind } from '@/api/system/setting'
import LegalDocumentEditor from '../LegalDocumentEditor/index.vue'

defineOptions({ name: 'LegalSettingsPanel' })

defineProps<{
  loading: boolean
  saving: boolean
  error: string
  canUpdate: boolean
}>()
const documents = defineModel<Record<LegalDocumentKind, string>>('documents', { required: true })
const emit = defineEmits<{ save: [kind: LegalDocumentKind] }>()
const { t } = useI18n()
const activeDocument = ref<LegalDocumentKind>('userAgreement')
</script>

<template>
  <section class="legal-settings" :aria-label="t('setting.legalTitle')">
    <header class="legal-settings__header">
      <div class="legal-settings__intro">
        <span class="legal-settings__eyebrow">{{ t('setting.legalTitle') }}</span>
        <h2>{{ t('setting.legalWorkspaceTitle') }}</h2>
        <p>{{ t('setting.legalWorkspaceDescription') }}</p>
      </div>
    </header>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-skeleton v-if="loading" :rows="8" animated />
    <div v-else class="legal-settings__workspace">
      <div class="legal-settings__document-bar">
        <el-tabs v-model="activeDocument" class="legal-settings__tabs">
          <el-tab-pane name="userAgreement" :label="t('setting.userAgreement')" />
          <el-tab-pane name="privacyPolicy" :label="t('setting.privacyPolicy')" />
        </el-tabs>
        <el-button
          v-if="canUpdate"
          data-testid="legal-document-save"
          type="primary"
          :icon="Check"
          :loading="saving"
          @click="emit('save', activeDocument)"
        >
          {{ t('setting.saveLegalDocument') }}
        </el-button>
      </div>
      <div class="legal-settings__editor-shell">
        <LegalDocumentEditor
          v-if="activeDocument === 'userAgreement'"
          v-model="documents.userAgreement"
          data-testid="user-agreement-editor"
          :disabled="!canUpdate"
        />
        <LegalDocumentEditor
          v-else
          v-model="documents.privacyPolicy"
          data-testid="privacy-policy-editor"
          :disabled="!canUpdate"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.legal-settings {
  width: min(1120px, 100%);
  margin: 0 auto;
}

.legal-settings__header {
  display: flex;
  align-items: flex-end;
  padding: 4px 0 20px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.legal-settings__eyebrow {
  display: block;
  margin-bottom: 8px;
  color: var(--el-color-primary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.legal-settings__intro h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 20px;
  font-weight: 600;
  line-height: 1.35;
}

.legal-settings__intro p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.legal-settings__workspace {
  margin-top: 24px;
  padding: 20px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-extra-light);
}

.legal-settings__document-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 16px;
}

.legal-settings__tabs {
  min-width: 0;
}

.legal-settings__tabs :deep(.el-tabs__header) {
  margin: 0;
}

.legal-settings__tabs :deep(.el-tabs__content) {
  display: none;
}

.legal-settings__editor-shell {
  padding: 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-bg-color);
}

@media (max-width: 760px) {
  .legal-settings__document-bar {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }

  .legal-settings__document-bar .el-button {
    width: 100%;
  }

  .legal-settings__workspace {
    padding: 12px;
  }

  .legal-settings__editor-shell {
    padding: 10px;
  }
}
</style>
