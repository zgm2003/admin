<script setup lang="ts">
import type { Schedule, TaskOption } from '@/api/system/scheduler'
defineProps<{ modelValue: boolean; schedule: Schedule | null; options: TaskOption[] }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>

<template>
  <AppDialog :model-value="modelValue" :title="$t(schedule ? 'scheduler.edit' : 'scheduler.create')">
    <slot />
  </AppDialog>
</template>
