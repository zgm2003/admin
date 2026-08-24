<script setup lang="ts">
import { Close, Moon, RefreshRight, Setting, Sunny } from '@element-plus/icons-vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useUIPreferencesStore } from '../../store/ui-preferences'
import type { PageTransitionName, UIPreferences } from '../../utils/ui-preferences'
import type { ThemeMode } from '../../utils/theme'

defineProps<{
  modelValue: boolean
  contentFullscreen: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const { t } = useI18n()
const uiPreferences = useUIPreferencesStore()
const themeColors = ['#409EFF', '#3B82F6', '#475569', '#059669', '#0891B2', '#7C3AED', '#EA580C'] as const
const transitionNames: readonly PageTransitionName[] = ['fade', 'slide-left', 'zoom']
type BooleanPreferenceKey = 'showBreadcrumb' | 'showMenuToggle' | 'showRouteTabs' | 'uniqueOpened' | 'showFooter' | 'pageTransition'

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

      <section class="setting-drawer__section">
        <h3>{{ t('layout.settings.theme') }}</h3>
        <div class="setting-drawer__segmented" role="group" :aria-label="t('layout.settings.theme')">
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
        </div>

        <div class="setting-drawer__color-label">{{ t('layout.settings.primaryColor') }}</div>
        <div class="setting-drawer__colors">
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
            <span v-if="uiPreferences.preferences.primaryColor === color" aria-hidden="true">✓</span>
          </button>
          <el-color-picker
            data-testid="primary-color-picker"
            :model-value="uiPreferences.preferences.primaryColor"
            :aria-label="t('layout.settings.primaryColor')"
            @change="handleColorChange"
          />
        </div>
      </section>

      <section class="setting-drawer__section">
        <h3>{{ t('layout.settings.display') }}</h3>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.breadcrumb') }}</span>
          <el-switch
            data-testid="show-breadcrumb"
            :model-value="uiPreferences.preferences.showBreadcrumb"
            @change="updateBoolean('showBreadcrumb', $event)"
          />
        </label>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.menuToggle') }}</span>
          <el-switch
            data-testid="show-menu-toggle"
            :model-value="uiPreferences.preferences.showMenuToggle"
            @change="updateBoolean('showMenuToggle', $event)"
          />
        </label>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.routeTabs') }}</span>
          <el-switch
            data-testid="show-route-tabs"
            :model-value="uiPreferences.preferences.showRouteTabs"
            :disabled="contentFullscreen"
            @change="updateBoolean('showRouteTabs', $event)"
          />
        </label>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.uniqueOpened') }}</span>
          <el-switch
            data-testid="unique-opened"
            :model-value="uiPreferences.preferences.uniqueOpened"
            @change="updateBoolean('uniqueOpened', $event)"
          />
        </label>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.footer') }}</span>
          <el-switch
            data-testid="show-footer"
            :model-value="uiPreferences.preferences.showFooter"
            @change="updateBoolean('showFooter', $event)"
          />
        </label>
      </section>

      <section class="setting-drawer__section">
        <h3>{{ t('layout.settings.transition') }}</h3>
        <label class="setting-drawer__row">
          <span>{{ t('layout.settings.transitionEnabled') }}</span>
          <el-switch
            data-testid="page-transition"
            :model-value="uiPreferences.preferences.pageTransition"
            @change="updateBoolean('pageTransition', $event)"
          />
        </label>
        <el-select
          data-testid="transition-name"
          :model-value="uiPreferences.preferences.transitionName"
          @change="updateTransitionName"
        >
          <el-option value="fade" :label="t('layout.settings.transitionFade')" />
          <el-option value="slide-left" :label="t('layout.settings.transitionSlideLeft')" />
          <el-option value="zoom" :label="t('layout.settings.transitionZoom')" />
        </el-select>
        <p v-if="contentFullscreen" class="setting-drawer__hint">
          {{ t('layout.settings.fullscreenTabsLocked') }}
        </p>
      </section>

      <div class="setting-drawer__footer">
        <el-button data-testid="reset-ui-preferences" :icon="RefreshRight" @click="resetPreferences">
          {{ t('layout.settings.reset') }}
        </el-button>
      </div>
    </div>
  </el-drawer>
</template>

<style scoped lang="scss">
.setting-drawer__content {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  gap: 18px;
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
  padding-bottom: 18px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.setting-drawer__section h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 14px;
}

.setting-drawer__segmented {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.setting-drawer__segmented .el-button {
  margin: 0;
}

.setting-drawer__color-label {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.setting-drawer__colors {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.setting-drawer__swatch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  color: white;
  border: 2px solid transparent;
  border-radius: 50%;
  cursor: pointer;
}

.setting-drawer__swatch.is-active {
  border-color: var(--el-text-color-primary);
  outline: 2px solid var(--el-color-primary-light-5);
  outline-offset: 1px;
}

.setting-drawer__row {
  justify-content: space-between;
  gap: 12px;
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.setting-drawer__hint {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.setting-drawer__footer {
  justify-content: flex-end;
  margin-top: auto;
}
</style>
