<script setup lang="ts">
import { Check, Close, Moon, RefreshRight, Setting, Sunny } from '@element-plus/icons-vue'
import { computed, markRaw } from 'vue'
import { useI18n } from 'vue-i18n'

import { useUIPreferencesStore } from '@/store/ui-preferences'
import type { PageTransitionName, UIPreferences } from '@/utils/ui-preferences'

defineOptions({ name: 'SettingDrawer' })

defineProps<{
  modelValue: boolean
  contentFullscreen: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const { t } = useI18n()
const uiPreferences = useUIPreferencesStore()
const themeColors = [
  '#409EFF',
  '#3B82F6',
  '#4F46E5',
  '#7C3AED',
  '#DB2777',
  '#DC2626',
  '#EA580C',
  '#CA8A04',
  '#059669',
  '#0891B2',
  '#475569',
] as const
const transitionNames: readonly PageTransitionName[] = ['fade', 'slide-left', 'zoom']
const transitionOptions = computed(() => [
  { value: 'fade', label: t('layout.settings.transitionFade') },
  { value: 'slide-left', label: t('layout.settings.transitionSlideLeft') },
  { value: 'zoom', label: t('layout.settings.transitionZoom') },
])
const themeModeOptions = computed(() => [
  { value: 'light', label: t('layout.settings.light'), icon: markRaw(Sunny), testId: 'theme-light' },
  { value: 'dark', label: t('layout.settings.dark'), icon: markRaw(Moon), testId: 'theme-dark' },
])
type BooleanPreferenceKey =
  | 'showBreadcrumb'
  | 'showMenuToggle'
  | 'showRouteTabs'
  | 'uniqueOpened'
  | 'showFooter'
  | 'pageTransition'

const persistenceErrorMessage = computed(() => {
  if (uiPreferences.persistenceError === 'invalid') return t('layout.settings.invalidStorage')
  if (uiPreferences.persistenceError === 'write') return t('layout.settings.writeFailed')
  return ''
})

function close(): void {
  emit('update:modelValue', false)
}

function updateTheme(value: string | number | boolean): void {
  if (value !== 'light' && value !== 'dark') {
    throw new Error(`Invalid theme mode: ${String(value)}`)
  }
  uiPreferences.update({ theme: value })
}

function updatePrimaryColor(color: string): void {
  uiPreferences.update({ primaryColor: color })
}

function handleColorChange(value: string | null): void {
  if (value !== null) updatePrimaryColor(value)
}

function updateBoolean(key: BooleanPreferenceKey, value: unknown): void {
  if (typeof value !== 'boolean') throw new Error(`Invalid boolean preference value for ${key}`)
  if (key === 'showMenuToggle' && !value && uiPreferences.preferences.showMenuToggle) {
    uiPreferences.update({ [key]: value } as Partial<UIPreferences>)
    return
  }
  uiPreferences.update({ [key]: value } as Partial<UIPreferences>)
}

function updateTransitionName(value: unknown): void {
  if (typeof value !== 'string' || !transitionNames.includes(value as PageTransitionName)) {
    throw new Error(`Invalid transition name: ${String(value)}`)
  }
  uiPreferences.update({ transitionName: value as PageTransitionName })
}

function resetPreferences(): void {
  uiPreferences.reset()
}
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    class="setting-drawer"
    direction="rtl"
    size="320px"
    :with-header="false"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="setting-drawer__content">
      <header class="setting-drawer__header">
        <div class="setting-drawer__title">
          <el-icon aria-hidden="true"><Setting /></el-icon>
          <span>{{ t('layout.settings.title') }}</span>
        </div>
        <el-button
          data-testid="close-settings"
          text
          :icon="Close"
          :aria-label="t('layout.settings.close')"
          @click="close"
        />
      </header>

      <el-alert
        v-if="uiPreferences.persistenceError !== null"
        data-testid="ui-preferences-error"
        type="error"
        :title="persistenceErrorMessage"
        :closable="false"
        show-icon
      />

      <section class="setting-drawer__section" data-testid="settings-theme-section">
        <h3>{{ t('layout.settings.theme') }}</h3>
        <el-segmented
          class="setting-drawer__segmented"
          :model-value="uiPreferences.preferences.theme"
          :options="themeModeOptions"
          :aria-label="t('layout.settings.theme')"
          @update:model-value="updateTheme"
        >
          <template #default="{ item }">
            <span class="setting-drawer__mode" :data-testid="item.testId">
              <el-icon aria-hidden="true"><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </span>
          </template>
        </el-segmented>

        <div class="setting-drawer__color-label">{{ t('layout.settings.primaryColor') }}</div>
        <div class="setting-drawer__colors">
          <button
            v-for="color in themeColors"
            :key="color"
            type="button"
            class="setting-drawer__swatch"
            :class="{ 'is-active': uiPreferences.preferences.primaryColor === color }"
            :data-testid="`primary-color-${color.slice(1)}`"
            :aria-label="color"
            :aria-pressed="uiPreferences.preferences.primaryColor === color"
            @click="updatePrimaryColor(color)"
          >
            <span class="setting-drawer__swatch-color" :style="{ backgroundColor: color }">
              <el-icon v-if="uiPreferences.preferences.primaryColor === color" aria-hidden="true">
                <Check />
              </el-icon>
            </span>
          </button>
          <el-color-picker
            data-testid="primary-color-picker"
            :model-value="uiPreferences.preferences.primaryColor"
            :aria-label="t('layout.settings.primaryColor')"
            @change="handleColorChange"
          />
        </div>
      </section>

      <section class="setting-drawer__section" data-testid="settings-display-section">
        <h3>{{ t('layout.settings.display') }}</h3>
        <div class="setting-drawer__toggles">
          <label
            v-for="item in [
              {
                key: 'showBreadcrumb',
                testId: 'show-breadcrumb',
                label: t('layout.settings.breadcrumb'),
              },
              {
                key: 'showMenuToggle',
                testId: 'show-menu-toggle',
                label: t('layout.settings.menuToggle'),
              },
              {
                key: 'showRouteTabs',
                testId: 'show-route-tabs',
                label: t('layout.settings.routeTabs'),
              },
              {
                key: 'uniqueOpened',
                testId: 'unique-opened',
                label: t('layout.settings.uniqueOpened'),
              },
              { key: 'showFooter', testId: 'show-footer', label: t('layout.settings.footer') },
            ]"
            :key="item.key"
            class="setting-drawer__row"
          >
            <span>{{ item.label }}</span>
            <el-switch
              :data-testid="item.testId"
              :model-value="uiPreferences.preferences[item.key as BooleanPreferenceKey]"
              :disabled="item.key === 'showRouteTabs' && contentFullscreen"
              @change="updateBoolean(item.key as BooleanPreferenceKey, $event)"
            />
          </label>
        </div>
      </section>

      <section class="setting-drawer__section" data-testid="settings-transition-section">
        <h3>{{ t('layout.settings.transition') }}</h3>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.transitionEnabled') }}</span>
          <el-switch
            data-testid="page-transition"
            :model-value="uiPreferences.preferences.pageTransition"
            @change="updateBoolean('pageTransition', $event)"
          />
        </label>
        <el-select-v2
          data-testid="transition-name"
          :model-value="uiPreferences.preferences.transitionName"
          :options="transitionOptions"
          @change="updateTransitionName"
        />
        <p v-if="contentFullscreen" class="setting-drawer__hint">
          {{ t('layout.settings.fullscreenTabsLocked') }}
        </p>
      </section>

      <div class="setting-drawer__footer">
        <el-button
          data-testid="reset-ui-preferences"
          :icon="RefreshRight"
          @click="resetPreferences"
        >
          {{ t('layout.settings.reset') }}
        </el-button>
      </div>
    </div>
  </el-drawer>
</template>

<style>
.setting-drawer .el-drawer__body {
  padding: 16px 18px;
}
</style>

<style scoped>
.setting-drawer__content {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  gap: 16px;
}

.setting-drawer__header,
.setting-drawer__title,
.setting-drawer__row,
.setting-drawer__footer {
  display: flex;
  align-items: center;
}

.setting-drawer__header {
  justify-content: space-between;
  min-height: 38px;
}

.setting-drawer__title {
  gap: 8px;
  color: var(--el-text-color-primary);
  font-weight: 700;
}

.setting-drawer__section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
}

.setting-drawer__section h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 14px;
  font-weight: 700;
}

.setting-drawer__segmented {
  width: 100%;
}

.setting-drawer__segmented :deep(.el-segmented__group) {
  width: 100%;
}

.setting-drawer__segmented :deep(.el-segmented__item) {
  flex: 1;
}

.setting-drawer__mode {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.setting-drawer__color-label {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.setting-drawer__colors {
  display: grid;
  grid-template-columns: repeat(4, 32px);
  justify-content: space-between;
  gap: 8px;
}

.setting-drawer__swatch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 4px;
  background: var(--el-fill-color-blank);
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  cursor: pointer;
  transition: border-color 0.2s;
}

.setting-drawer__swatch:hover {
  border-color: var(--el-text-color-placeholder);
}

.setting-drawer__swatch.is-active {
  border-color: var(--el-color-primary);
  outline: 2px solid var(--el-color-primary-light-5);
  outline-offset: 1px;
}

.setting-drawer__swatch-color {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: #fff;
  border-radius: calc(var(--el-border-radius-base) - 1px);
}

.setting-drawer__toggles {
  display: flex;
  flex-direction: column;
}

.setting-drawer__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 36px;
  gap: 12px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
}

.setting-drawer__toggles .setting-drawer__row + .setting-drawer__row {
  border-top: 1px solid var(--el-border-color-extra-light);
}

.setting-drawer__hint {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.setting-drawer__footer {
  justify-content: flex-end;
  padding-top: 4px;
  margin-top: auto;
}

.setting-drawer__footer .el-button {
  width: 100%;
}

.setting-drawer__section :deep(.el-select) {
  width: 100%;
}
</style>
