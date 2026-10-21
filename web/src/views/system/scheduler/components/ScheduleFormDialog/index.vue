<!-- 组件：定时任务计划新建与编辑弹窗 -->
<script setup lang="ts">
import type { Schedule, TaskOption } from '@/api/system/scheduler'
defineProps<{ modelValue: boolean; schedule: Schedule | null; options: TaskOption[] }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>

<template>
  <AppDialog :model-value="modelValue" :title="$t(schedule ? 'common.edit' : 'scheduler.create')">
    <slot />
  </AppDialog>
</template>
