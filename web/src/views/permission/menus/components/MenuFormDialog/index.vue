<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ManagedMenuType, MenuPlatformOption } from '@/api/permission/menu'
import { AppDIcon } from '@/components/AppDIcon'
import { AppDialog } from '@/components/AppDialog'
import { IconSelect } from '@/components/IconSelect'
import { YesNo } from '@/enums/yes-no'
import type { MenuIconName } from '@/icons/menu-icons'
import type { MenuFormState } from '@/views/permission/menus/components/types'

const props = defineProps<{
  dialogMode: 'create' | 'edit'
  mutationError: string
  editingProtected: boolean
  activePlatform: MenuPlatformOption | null
  canSubmitForm: boolean
  parentSelectOptions: Array<{ label: string; value: number | '__root__' }>
  menuTypeOptions: Array<{ label: string; value: ManagedMenuType }>
  handleFormTypeChange: (nextType: ManagedMenuType) => void
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<MenuFormState>('form', { required: true })
const parentSelection = defineModel<number | '__root__'>('parentSelection', { required: true })
const emit = defineEmits<{ close: []; save: [] }>()
const { t } = useI18n()
const iconSelectVisible = ref(false)

function selectMenuIcon(value: MenuIconName): void {
  form.value.icon = value
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="props.dialogMode === 'create' ? t('menu.form.createTitle') : t('menu.form.editTitle')"
    width="900px"
    data-testid="menu-dialog"
  >
    <el-alert
      v-if="props.mutationError !== ''"
      data-testid="menu-form-error"
      type="error"
      :title="props.mutationError"
      :closable="false"
      show-icon
    />
    <el-alert
      v-if="props.editingProtected"
      data-testid="menu-form-protected-hint"
      type="info"
      :title="t('menu.form.protectedHint')"
      :closable="false"
      show-icon
    />
    <el-form
      class="menu-form"
      label-position="right"
      label-width="96px"
      @submit.prevent="emit('save')"
    >
      <el-form-item v-if="props.dialogMode === 'edit'" :label="t('menu.form.platform')">
        <div data-testid="menu-form-platform" class="menu-form__readonly">
          <span>{{ props.activePlatform?.name }}</span>
          <code>{{ props.activePlatform?.code }}</code>
        </div>
      </el-form-item>

      <el-form-item :label="t('menu.form.code')">
        <div class="menu-form__control">
          <el-input
            v-model="form.code"
            data-testid="menu-form-code"
            :readonly="props.dialogMode === 'edit'"
            :disabled="props.editingProtected"
            :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
            :placeholder="t('menu.form.codePlaceholder')"
          />
          <p class="menu-form__hint">{{ t('menu.form.codeHint') }}</p>
        </div>
      </el-form-item>

      <el-form-item :label="t('menu.form.name')">
        <el-input v-model="form.name" data-testid="menu-form-name" maxlength="128" />
      </el-form-item>

      <el-form-item :label="t('menu.form.remark')">
        <el-input
          v-model="form.remark"
          data-testid="menu-form-remark"
          type="textarea"
          :rows="3"
          maxlength="512"
          show-word-limit
          :placeholder="t('menu.form.remarkPlaceholder')"
        />
      </el-form-item>

      <el-form-item v-if="form.menuType !== 'action'" :label="t('menu.form.i18nKey')">
        <div class="menu-form__control">
          <el-input v-model="form.i18nKey" data-testid="menu-form-i18n-key" />
          <p class="menu-form__hint">{{ t('menu.form.i18nKeyHint') }}</p>
        </div>
      </el-form-item>

      <el-form-item v-if="form.menuType === 'page'" :label="t('menu.form.path')">
        <div class="menu-form__control">
          <el-input
            v-model="form.path"
            data-testid="menu-form-path"
            :disabled="props.editingProtected"
            :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
            :placeholder="t('menu.form.pathPlaceholder')"
          />
          <p class="menu-form__hint">{{ t('menu.form.pathHint') }}</p>
        </div>
      </el-form-item>

      <el-form-item v-if="form.menuType === 'page'" :label="t('menu.form.componentPath')">
        <div class="menu-form__control">
          <el-input
            v-model="form.componentPath"
            data-testid="menu-form-component-path"
            :disabled="props.editingProtected"
            :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
            :placeholder="t('menu.form.componentPathPlaceholder')"
          />
          <p class="menu-form__hint">{{ t('menu.form.componentPathHint') }}</p>
        </div>
      </el-form-item>

      <el-row :gutter="24" class="menu-form__grid">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.parent')">
            <el-select-v2
              v-model="parentSelection"
              data-testid="menu-form-parent"
              clearable
              :disabled="props.dialogMode === 'edit' || props.editingProtected"
              :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
              :options="props.parentSelectOptions"
            />
          </el-form-item>
        </el-col>

        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.menuType')">
            <el-select-v2
              :model-value="form.menuType"
              data-testid="menu-form-type"
              :disabled="props.dialogMode === 'edit' || props.editingProtected"
              :class="{ 'is-disabled': props.dialogMode === 'edit' || props.editingProtected }"
              :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
              :options="props.menuTypeOptions"
              @update:model-value="props.handleFormTypeChange"
            />
          </el-form-item>
        </el-col>

        <el-col :xs="24" :sm="12">
          <el-form-item v-if="form.menuType !== 'action'" :label="t('menu.form.icon')">
            <div class="menu-icon-picker">
              <el-button
                data-testid="menu-form-icon"
                :disabled="props.editingProtected"
                @click="iconSelectVisible = true"
              >
                <AppDIcon v-if="form.icon !== null" :icon="form.icon" :size="24" />
                <span v-else>{{ t('menu.form.selectIcon') }}</span>
              </el-button>
              <el-button
                v-if="form.icon !== null"
                text
                type="danger"
                :disabled="props.editingProtected"
                @click="form.icon = null"
                >{{ t('menu.form.clearIcon') }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>

        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.sortOrder')">
            <el-input-number
              v-model="form.sortOrder"
              data-testid="menu-form-sort-order"
              :min="0"
              :step="10"
            />
          </el-form-item>
        </el-col>

        <el-col v-if="props.dialogMode === 'create'" :xs="24" :sm="12">
          <el-form-item v-if="props.dialogMode === 'create'" :label="t('menu.form.isEnabled')">
            <el-switch
              v-model="form.isEnabled"
              :active-value="YesNo.Yes"
              :inactive-value="YesNo.No"
              data-testid="menu-form-enabled"
            />
          </el-form-item>
        </el-col>

        <el-col v-if="form.menuType !== 'action'" :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.isHidden')">
            <el-switch
              v-model="form.isHidden"
              :active-value="YesNo.No"
              :inactive-value="YesNo.Yes"
              :disabled="props.editingProtected"
              :title="props.editingProtected ? t('menu.form.protectedHint') : undefined"
              data-testid="menu-form-hidden"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <template #footer>
      <div class="menu-form-actions">
        <el-button data-testid="menu-form-cancel" @click="emit('close')"
          >{{ t('menu.form.cancel') }}
        </el-button>
        <el-button
          data-testid="menu-form-submit"
          type="primary"
          :disabled="!props.canSubmitForm"
          @click="emit('save')"
        >
          {{ t('menu.form.submit') }}
        </el-button>
      </div>
    </template>
  </AppDialog>

  <IconSelect
    v-model="iconSelectVisible"
    :title="t('menu.form.icon')"
    :empty-text="t('menu.form.noMatchingIcon')"
    @select-icon="selectMenuIcon"
  />
</template>

<style scoped>
.menu-form__readonly {
  display: inline-flex;
  min-width: 0;
  min-height: 32px;
  align-items: center;
  gap: 7px;
  color: var(--admin-text);
}
.menu-form__readonly code {
  color: var(--admin-text-soft);
  font-family: Consolas, 'SFMono-Regular', monospace;
  font-size: 12px;
}
.menu-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.menu-form__grid,
.menu-form__control,
.menu-icon-picker {
  width: 100%;
}
.menu-form__grid :deep(.el-input),
.menu-form__grid :deep(.el-select),
.menu-form__grid :deep(.el-input-number) {
  width: 100%;
}
.menu-icon-picker {
  display: flex;
  align-items: center;
  gap: 6px;
}
.menu-icon-picker .el-button:first-child {
  flex: 1;
  justify-content: center;
}
.menu-form__hint {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}
</style>
