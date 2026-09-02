<script setup lang="ts">
import { Close, Moon, RefreshRight, Setting, Sunny } from '@element-plus/icons-vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useUIPreferencesStore } from '@/store/ui-preferences'
import type { PageTransitionName, UIPreferences } from '@/utils/ui-preferences'
import type { ThemeMode } from '@/utils/theme'

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
  '#475569',
  '#059669',
  '#0891B2',
  '#7C3AED',
  '#EA580C',
] as const
const transitionNames: readonly PageTransitionName[] = ['fade', 'slide-left', 'zoom']
const transitionOptions = computed(() => [
  { value: 'fade', label: t('layout.settings.transitionFade') },
  { value: 'slide-left', label: t('layout.settings.transitionSlideLeft') },
  { value: 'zoom', label: t('layout.settings.transitionZoom') },
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

function updateTheme(theme: ThemeMode): void {
  uiPreferences.update({ theme })
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
        <el-space
          class="setting-drawer__segmented"
          wrap
          fill
          :size="8"
          role="group"
          :aria-label="t('layout.settings.theme')"
        >
          <el-button
            data-testid="theme-light"
            :type="uiPreferences.preferences.theme === 'light' ? 'primary' : 'default'"
            :plain="uiPreferences.preferences.theme !== 'light'"
            :icon="Sunny"
            @click="updateTheme('light')"
          >
            {{ t('layout.settings.light') }}
          </el-button>
          <el-button
            data-testid="theme-dark"
            :type="uiPreferences.preferences.theme === 'dark' ? 'primary' : 'default'"
            :plain="uiPreferences.preferences.theme !== 'dark'"
            :icon="Moon"
            @click="updateTheme('dark')"
          >
            {{ t('layout.settings.dark') }}
          </el-button>
        </el-space>

        <div class="setting-drawer__color-label">{{ t('layout.settings.primaryColor') }}</div>
        <el-space class="setting-drawer__colors" wrap :size="10">
          <button
            v-for="color in themeColors"
            :key="color"
            type="button"
            class="setting-drawer__swatch"
            :class="{ 'is-active': uiPreferences.preferences.primaryColor === color }"
            :data-testid="`primary-color-${color.slice(1)}`"
            :style="{ backgroundColor: color }"
            :aria-label="color"
            :aria-pressed="uiPreferences.preferences.primaryColor === color"
            @click="updatePrimaryColor(color)"
          >
            <span v-if="uiPreferences.preferences.primaryColor === color" aria-hidden="true"
              >✓</span
            >
          </button>
          <el-color-picker
            data-testid="primary-color-picker"
            :model-value="uiPreferences.preferences.primaryColor"
            :aria-label="t('layout.settings.primaryColor')"
            @change="handleColorChange"
          />
        </el-space>
      </section>

      <section class="setting-drawer__section" data-testid="settings-display-section">
        <h3>{{ t('layout.settings.display') }}</h3>
        <el-row :gutter="8" class="setting-drawer__display-grid">
          <el-col
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
            :xs="24"
            :sm="12"
          >
            <label class="setting-drawer__row">
              <span>{{ item.label }}</span>
              <el-switch
                :data-testid="item.testId"
                :model-value="uiPreferences.preferences[item.key as BooleanPreferenceKey]"
                :disabled="item.key === 'showRouteTabs' && contentFullscreen"
                @change="updateBoolean(item.key as BooleanPreferenceKey, $event)"
              />
            </label>
          </el-col>
        </el-row>
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
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-fill-color-extra-light);
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

.setting-drawer__segmented .el-button {
  margin: 0;
  min-height: 68px;
  padding: 10px;
  border-radius: 6px;
  flex-direction: column;
  gap: 6px;
}

.setting-drawer__color-label {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.setting-drawer__colors {
  width: 100%;
}

.setting-drawer__swatch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 32px;
  padding: 0;
  color: white;
  border: 2px solid transparent;
  border-radius: 6px;
  cursor: pointer;
}

.setting-drawer__swatch.is-active {
  border-color: var(--el-text-color-primary);
  outline: 2px solid var(--el-color-primary-light-5);
  outline-offset: 1px;
}

.setting-drawer__row {
  min-height: 38px;
  justify-content: space-between;
  gap: 12px;
  padding: 0 10px;
  border-radius: 6px;
  background: var(--el-bg-color);
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.setting-drawer__display-grid {
  width: 100%;
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

.setting-drawer__colors :deep(.el-color-picker),
.setting-drawer__colors :deep(.el-color-picker__trigger) {
  width: 100%;
  height: 32px;
}

.setting-drawer__colors :deep(.el-space__item) {
  flex: 1 0 48px;
}

.setting-drawer__colors :deep(.el-color-picker__trigger) {
  padding: 2px;
  border-radius: 6px;
}

:deep(.setting-drawer .el-drawer__body) {
  padding: 18px;
}
</style>
