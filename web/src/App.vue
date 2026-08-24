<script setup lang="ts">
import { useUIPreferencesStore } from './store/ui-preferences'

const uiPreferences = useUIPreferencesStore()
if (!uiPreferences.initialized) uiPreferences.initializeSafely()
</script>

<template>
  <RouterView />
</template>
