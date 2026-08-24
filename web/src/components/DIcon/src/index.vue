<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { Component } from 'vue'
import { computed, markRaw, ref, shallowRef, toRaw, watch } from 'vue'
import { ElIcon } from 'element-plus'
import type { DIconProps } from './types'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<DIconProps>(), {
  size: 18,
  color: undefined,
})

type ElementPlusIconsModule = typeof import('@element-plus/icons-vue')
type ElementPlusIconName = keyof ElementPlusIconsModule

const resolvedComponent = shallowRef<Component | null>(null)
const invalidMessage = ref('')

const isIconify = computed(() => typeof props.icon === 'string' && props.icon.includes(':'))
const iconStyle = computed(() => ({
  width: typeof props.size === 'number' ? `${props.size}px` : props.size,
  height: typeof props.size === 'number' ? `${props.size}px` : props.size,
  color: props.color,
}))

function hasElementPlusIcon(mod: ElementPlusIconsModule, name: string): name is ElementPlusIconName {
  return Object.prototype.hasOwnProperty.call(mod, name)
}

function reportInvalid(message: string): void {
  invalidMessage.value = message
  if (import.meta.env.DEV) {
    console.error(`[DIcon] ${message}`)
  }
}

async function resolveIcon(name: string): Promise<Component | null> {
  const module = await import('@element-plus/icons-vue')
  return hasElementPlusIcon(module, name) ? module[name] : null
}

watch(
  () => [props.icon, props.component] as const,
  async ([icon, component]) => {
    resolvedComponent.value = null
    invalidMessage.value = ''

    if (icon !== undefined && component !== undefined) {
      reportInvalid('icon and component cannot be provided together')
      return
    }
    if (component !== undefined) {
      resolvedComponent.value = markRaw(toRaw(component))
      return
    }
    if (icon === undefined || icon.trim() === '') {
      reportInvalid('an icon name or component is required')
      return
    }
    if (isIconify.value) return

    const resolved = await resolveIcon(icon)
    if (resolved === null) {
      reportInvalid(`Element Plus icon not found: ${icon}`)
      return
    }
    resolvedComponent.value = resolved
  },
  { immediate: true },
)
</script>

<template>
  <span
    v-bind="$attrs"
    class="d-icon"
    :style="iconStyle"
    data-testid="d-icon"
  >
    <Icon
      v-if="isIconify && props.icon"
      :icon="props.icon"
      width="100%"
      height="100%"
    />
    <ElIcon v-else-if="resolvedComponent">
      <component :is="resolvedComponent" />
    </ElIcon>
    <span v-else data-testid="d-icon-empty" :title="invalidMessage">?</span>
  </span>
</template>

<style scoped>
.d-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  vertical-align: middle;
}

.d-icon-empty {
  font-size: 0.75em;
  line-height: 1;
}
</style>
