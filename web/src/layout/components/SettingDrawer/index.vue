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

<style scoped src="./SettingDrawer.css"></style>
