<script setup lang="ts">
import type { Component } from 'vue'
import { computed, markRaw, toRaw } from 'vue'
import { ElIcon } from 'element-plus'
import { isMenuIconName, menuIcons, type MenuIconName } from '../../../icons/menu-icons'
import type { DIconProps } from './types'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<DIconProps>(), {
  size: 18,
  color: undefined,
})

const iconStyle = computed(() => ({
  width: typeof props.size === 'number' ? `${props.size}px` : props.size,
  height: typeof props.size === 'number' ? `${props.size}px` : props.size,
  color: props.color,
}))

const resolvedComponent = computed<Component | null>(() => {
  if (props.component !== undefined) return markRaw(toRaw(props.component))
  if (props.icon === undefined || !isMenuIconName(props.icon)) return null
  return menuIcons[props.icon as MenuIconName].component
})

const invalidMessage = computed(() => {
  if (props.component === undefined && props.icon === undefined) return 'an icon name or component is required'
  return resolvedComponent.value === null ? `Lucide icon not found: ${String(props.icon)}` : ''
})
</script>

<template>
  <span v-bind="$attrs" class="d-icon" :style="iconStyle" data-testid="d-icon">
    <ElIcon v-if="resolvedComponent">
      <component :is="resolvedComponent" />
    </ElIcon>
    <span v-else data-testid="d-icon-empty" :title="invalidMessage">?</span>
  </span>
</template>

<style scoped>
.d-icon { display: inline-flex; align-items: center; justify-content: center; vertical-align: middle; }
.d-icon-empty { font-size: 0.75em; line-height: 1; }
</style>
