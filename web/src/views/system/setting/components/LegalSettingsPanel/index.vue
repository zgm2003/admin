<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { LegalDocumentKind } from '@/api/system/setting'
import LegalDocumentEditor from '@/views/system/setting/components/LegalDocumentEditor/index.vue'

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
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-skeleton v-if="loading" :rows="8" animated />
    <template v-else>
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
      <LegalDocumentEditor
        v-if="activeDocument === 'userAgreement'"
        v-model="documents.userAgreement"
        class="legal-settings__editor"
        data-testid="user-agreement-editor"
        :disabled="!canUpdate"
      />
      <LegalDocumentEditor
        v-else
        v-model="documents.privacyPolicy"
        class="legal-settings__editor"
        data-testid="privacy-policy-editor"
        :disabled="!canUpdate"
      />
    </template>
  </section>
</template>

<style scoped>
.legal-settings {
  display: flex;
  width: min(1120px, 100%);
  height: 100%;
  min-height: 0;
  margin: 0 auto;
  flex-direction: column;
  gap: 12px;
}

.legal-settings__document-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
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

.legal-settings__editor {
  min-height: 0;
  flex: 1 1 auto;
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
}
</style>
