<script setup lang="ts">
import { Check, Moon, Sunny } from '@element-plus/icons-vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { LocaleSwitch } from '@/components/LocaleSwitch'
import { useUIPreferencesStore } from '@/store/uiPreferences'
import { themeColorPresets } from '@/utils/theme'

defineOptions({ name: 'AuthDock' })

const { t } = useI18n()
const uiPreferences = useUIPreferencesStore()
const isDark = computed(() => uiPreferences.preferences.theme === 'dark')

function toggleTheme(): void {
  uiPreferences.update({ theme: isDark.value ? 'light' : 'dark' })
}

function setPrimaryColor(color: string): void {
  uiPreferences.update({ primaryColor: color })
}
</script>

<template>
  <div class="auth-dock" data-testid="auth-dock">
    <LocaleSwitch />
    <el-tooltip :content="t('auth.dock.theme')" placement="bottom">
      <el-button
        data-testid="auth-theme-toggle"
        text
        :icon="isDark ? Sunny : Moon"
        :aria-label="t('auth.dock.theme')"
        @click="toggleTheme"
      />
    </el-tooltip>
    <el-popover placement="bottom-end" trigger="click" :width="236">
      <template #reference>
        <el-button
          data-testid="auth-color-trigger"
          text
          :aria-label="t('layout.settings.primaryColor')"
        >
          <span
            class="auth-dock__swatch"
            :style="{ backgroundColor: uiPreferences.preferences.primaryColor }"
          ></span>
        </el-button>
      </template>
      <p class="auth-dock__colors-title">{{ t('layout.settings.primaryColor') }}</p>
      <div class="auth-dock__colors">
        <button
          v-for="color in themeColorPresets"
          :key="color"
          type="button"
          class="auth-dock__color"
          :class="{ 'is-active': uiPreferences.preferences.primaryColor === color }"
          :data-testid="`auth-color-${color.slice(1)}`"
          :aria-label="color"
          :aria-pressed="uiPreferences.preferences.primaryColor === color"
          @click="setPrimaryColor(color)"
        >
          <span class="auth-dock__color-fill" :style="{ backgroundColor: color }">
            <el-icon v-if="uiPreferences.preferences.primaryColor === color" aria-hidden="true"
              ><Check
            /></el-icon>
          </span>
        </button>
      </div>
    </el-popover>
  </div>
</template>

<style scoped>
.auth-dock {
  position: absolute;
  top: 18px;
  right: 20px;
  z-index: 10;
  display: flex;
  align-items: center;
  padding: 5px 8px;
  gap: 2px;
  background: color-mix(in srgb, var(--el-bg-color) 78%, transparent);
  border: 1px solid var(--el-border-color-light);
  border-radius: 999px;
  box-shadow: var(--admin-shadow-sm);
  backdrop-filter: blur(12px);
  animation: auth-dock-in 0.5s 0.3s cubic-bezier(0.22, 0.8, 0.32, 1) backwards;
}

.auth-dock__swatch {
  display: block;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px rgb(255 255 255 / 55%);
}

.auth-dock__colors-title {
  margin: 0 0 10px;
  color: var(--el-text-color-primary);
  font-size: 13px;
  font-weight: 650;
}

.auth-dock__colors {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 8px;
}

.auth-dock__color {
  padding: 3px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  transition:
    transform 0.15s ease,
    border-color 0.15s ease;
}

.auth-dock__color:hover {
  transform: scale(1.08);
}

.auth-dock__color.is-active {
  border-color: var(--el-color-primary);
}

.auth-dock__color:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}

.auth-dock__color-fill {
  display: grid;
  width: 100%;
  place-items: center;
  color: #fff;
  border-radius: 6px;
  font-size: 13px;
  aspect-ratio: 1;
}

@keyframes auth-dock-in {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-dock {
    animation: none;
  }
}
</style>
