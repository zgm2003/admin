<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

const visible = defineModel<boolean>({ required: true });

const props = defineProps<{
  addedLabels: string[];
  removedLabels: string[];
  saving: boolean;
  error: string;
}>();

const emit = defineEmits<{
  confirm: [];
}>();

const { t } = useI18n();
const hasAdded = computed(() => props.addedLabels.length > 0);
const hasRemoved = computed(() => props.removedLabels.length > 0);
</script>

<template>
  <el-dialog
    v-model="visible"
    class="role-permission-diff-dialog"
    width="min(560px, 94vw)"
    append-to-body
  >
    <template #header>
      <strong>{{ t("role.permission.confirmTitle") }}</strong>
    </template>

    <el-alert v-if="error" :title="error" type="error" show-icon />
    <div class="role-permission-diff">
      <section class="role-permission-diff__section">
        <div class="role-permission-diff__title">
          {{ t("role.permission.added") }}
        </div>
        <el-empty
          v-if="!hasAdded"
          :description="t('role.permission.noChanges')"
          :image-size="60"
        />
        <template v-else>
          <el-tag
            v-for="label in addedLabels"
            :key="label"
            type="success"
            class="role-permission-diff__tag"
          >
            {{ label }}
          </el-tag>
        </template>
      </section>

      <section class="role-permission-diff__section">
        <div class="role-permission-diff__title">
          {{ t("role.permission.removed") }}
        </div>
        <el-empty
          v-if="!hasRemoved"
          :description="t('role.permission.noChanges')"
          :image-size="60"
        />
        <template v-else>
          <el-tag
            v-for="label in removedLabels"
            :key="label"
            type="danger"
            class="role-permission-diff__tag"
          >
            {{ label }}
          </el-tag>
        </template>
      </section>
    </div>

    <template #footer>
      <el-button :disabled="saving" @click="visible = false">
        {{ t("role.confirm.cancel") }}
      </el-button>
      <el-button type="primary" :loading="saving" @click="emit('confirm')">
        {{ t("role.confirm.confirm") }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.role-permission-diff {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 12px;
}

.role-permission-diff__section {
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
}

.role-permission-diff__title {
  margin-bottom: 10px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.role-permission-diff__tag {
  margin-right: 8px;
  margin-bottom: 8px;
}
</style>
