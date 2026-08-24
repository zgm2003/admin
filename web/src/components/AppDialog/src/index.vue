<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useAttrs, useId, watch } from 'vue'
import {
  DEFAULT_APP_DIALOG_MOBILE_WIDTH,
  filterAppDialogAttrs,
  resolveAppDialogAlignCenter,
  resolveAppDialogBodyPadding,
  resolveAppDialogContentHeight,
  resolveAppDialogDraggable,
  resolveAppDialogPadding,
  resolveAppDialogWidth,
  type AppDialogSize,
} from './dialog'

defineOptions({ name: 'AppDialog', inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  ariaLabel?: string
  description?: string
  width?: AppDialogSize
  mobileWidth?: AppDialogSize
  height?: AppDialogSize
  bodyPadding?: AppDialogSize
  headerPadding?: AppDialogSize
  footerPadding?: AppDialogSize
  showHeader?: boolean
  appendToBody?: boolean
  destroyOnClose?: boolean
  draggable?: boolean
  top?: string
  showClose?: boolean
  alignCenter?: boolean
  closeOnPressEscape?: boolean
}>(), {
  title: '',
  ariaLabel: '',
  description: '',
  width: undefined,
  mobileWidth: DEFAULT_APP_DIALOG_MOBILE_WIDTH,
  height: undefined,
  bodyPadding: undefined,
  headerPadding: undefined,
  footerPadding: undefined,
  showHeader: true,
  appendToBody: true,
  destroyOnClose: true,
  draggable: undefined,
  top: '5vh',
  showClose: true,
  alignCenter: false,
  closeOnPressEscape: true,
})

const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const attrs = useAttrs()
const isMobile = ref(false)
const descriptionId = `app-dialog-description-${useId()}`
let returnFocusTarget: HTMLElement | null = null

function updateMobile(): void {
  isMobile.value = window.innerWidth <= 768
}

onMounted(() => {
  updateMobile()
  window.addEventListener('resize', updateMobile)
})
onBeforeUnmount(() => window.removeEventListener('resize', updateMobile))

watch(() => props.modelValue, (visible, previousVisible) => {
  if (visible && !previousVisible && document.activeElement instanceof HTMLElement) {
    returnFocusTarget = document.activeElement
  }
})

function restoreTriggerFocus(): void {
  const target = returnFocusTarget
  returnFocusTarget = null
  if (target === null) return
  void nextTick(() => target.focus())
}

const dialogAttrs = computed(() => {
  const filtered = filterAppDialogAttrs(attrs)
  delete filtered.class
  delete filtered.style
  return filtered
})
const dialogStyle = computed(() => ({
  ...(resolveAppDialogPadding(props.headerPadding)
    ? { '--app-dialog-header-padding': resolveAppDialogPadding(props.headerPadding) }
    : {}),
  ...(resolveAppDialogPadding(props.footerPadding)
    ? { '--app-dialog-footer-padding': resolveAppDialogPadding(props.footerPadding) }
    : {}),
}))
const dialogClasses = computed(() => [
  'app-dialog',
  attrs.class,
  {
    'app-dialog--header-hidden': !props.showHeader,
    'app-dialog--custom-header-padding': props.headerPadding !== undefined,
    'app-dialog--custom-footer-padding': props.footerPadding !== undefined,
  },
])
const resolvedTitle = computed(() => props.showHeader ? props.title : props.ariaLabel || props.title)
const bodyStyle = computed(() => ({
  padding: resolveAppDialogBodyPadding({ isMobile: isMobile.value, bodyPadding: props.bodyPadding }),
}))
</script>

<template>
  <el-dialog
    v-bind="dialogAttrs"
    :model-value="modelValue"
    :title="resolvedTitle"
    :width="resolveAppDialogWidth({ isMobile, width, mobileWidth })"
    :append-to-body="appendToBody"
    :destroy-on-close="destroyOnClose"
    :draggable="resolveAppDialogDraggable({ isMobile, draggable })"
    :top="top"
    :show-close="showClose"
    :align-center="resolveAppDialogAlignCenter({ isMobile, alignCenter })"
    :close-on-press-escape="closeOnPressEscape"
    :aria-label="ariaLabel || undefined"
    :aria-describedby="description ? descriptionId : undefined"
    :class="dialogClasses"
    :style="[attrs.style, dialogStyle]"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="restoreTriggerFocus"
  >
    <p v-if="description" :id="descriptionId" class="app-dialog__sr-only">{{ description }}</p>
    <template v-if="showHeader && ($slots.header || (!title && ariaLabel))" #header="{ titleId, titleClass }">
      <div v-if="$slots.header" :id="titleId" class="app-dialog__header-content"><slot name="header" /></div>
      <span v-else :id="titleId" :class="[titleClass, 'app-dialog__sr-only']">{{ ariaLabel }}</span>
    </template>
    <div v-if="resolveAppDialogContentHeight(height)" class="app-dialog__body app-dialog__body--scroll">
      <el-scrollbar :height="resolveAppDialogContentHeight(height)" class="app-dialog__scrollbar">
        <div class="app-dialog__content" :style="bodyStyle"><slot /></div>
      </el-scrollbar>
    </div>
    <div v-else class="app-dialog__body"><div class="app-dialog__content" :style="bodyStyle"><slot /></div></div>
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </el-dialog>
</template>

<style scoped>
.app-dialog :deep(.el-dialog__body) { padding: 0; }
.app-dialog--header-hidden :deep(.el-dialog__header) { display: none; }
.app-dialog--custom-header-padding :deep(.el-dialog__header) { padding: var(--app-dialog-header-padding); }
.app-dialog--custom-footer-padding :deep(.el-dialog__footer) { padding: var(--app-dialog-footer-padding); }
.app-dialog__body, .app-dialog__content, .app-dialog__header-content, .app-dialog__scrollbar { width: 100%; }
.app-dialog__body--scroll :deep(.el-scrollbar__wrap) { overflow-x: hidden; }
.app-dialog__sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
@media (max-width: 768px) { .app-dialog :deep(.el-dialog) { margin: 3vh auto !important; left: 0 !important; right: 0 !important; transform: none !important; } }
</style>
