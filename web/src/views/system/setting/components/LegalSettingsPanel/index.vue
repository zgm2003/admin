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
    <header v-if="canUpdate" class="legal-settings__header">
      <el-button
        data-testid="legal-document-save"
        type="primary"
        :icon="Check"
        :loading="saving"
        @click="emit('save', activeDocument)"
      >
        {{ t('setting.saveLegalDocument') }}
      </el-button>
    </header>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-skeleton v-if="loading" :rows="8" animated />
    <el-tabs v-else v-model="activeDocument" class="legal-settings__tabs">
      <el-tab-pane name="userAgreement" :label="t('setting.userAgreement')">
        <LegalDocumentEditor
          v-model="documents.userAgreement"
          data-testid="user-agreement-editor"
          :disabled="!canUpdate"
        />
      </el-tab-pane>
      <el-tab-pane name="privacyPolicy" :label="t('setting.privacyPolicy')">
        <LegalDocumentEditor
          v-model="documents.privacyPolicy"
          data-testid="privacy-policy-editor"
          :disabled="!canUpdate"
        />
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<style scoped>
.legal-settings {
  width: min(1040px, 100%);
}

.legal-settings__header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.legal-settings__tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

@media (max-width: 760px) {
  .legal-settings__header .el-button {
    width: 100%;
  }
}
</style>
