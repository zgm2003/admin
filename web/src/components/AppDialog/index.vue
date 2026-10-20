<!-- 组件：通用弹窗 -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useAttrs, useId, watch } from 'vue'
import {
  DEFAULT_APP_DIALOG_MOBILE_WIDTH,
  filterAppDialogAttrs,
  resolveAppDialogBodyPadding,
  resolveAppDialogDraggable,
  resolveAppDialogPadding,
  resolveAppDialogWidth,
  type AppDialogSize,
} from './dialog'

defineOptions({ name: 'AppDialog', inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title?: string
    ariaLabel?: string
    description?: string
    width?: AppDialogSize
    mobileWidth?: AppDialogSize
    bodyPadding?: AppDialogSize
    headerPadding?: AppDialogSize
    footerPadding?: AppDialogSize
    showHeader?: boolean
    appendToBody?: boolean
    destroyOnClose?: boolean
    draggable?: boolean
    top?: string
    showClose?: boolean
    center?: boolean
    closeOnPressEscape?: boolean
  }>(),
  {
    title: '',
    ariaLabel: '',
    description: '',
    width: undefined,
    mobileWidth: DEFAULT_APP_DIALOG_MOBILE_WIDTH,
    bodyPadding: undefined,
    headerPadding: undefined,
    footerPadding: undefined,
    showHeader: true,
    appendToBody: true,
    destroyOnClose: true,
    draggable: undefined,
    top: '5vh',
    showClose: true,
    center: true,
    closeOnPressEscape: true,
  },
)

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

watch(
  () => props.modelValue,
  (visible, previousVisible) => {
    if (visible && !previousVisible && document.activeElement instanceof HTMLElement) {
      returnFocusTarget = document.activeElement
    }
  },
  { flush: 'sync' },
)

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
const resolvedTitle = computed(() =>
  props.showHeader ? props.title : props.ariaLabel || props.title,
)
const bodyStyle = computed(() => ({
  boxSizing: 'border-box' as const,
  padding: resolveAppDialogBodyPadding({
    isMobile: isMobile.value,
    bodyPadding: props.bodyPadding,
  }),
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
    :align-center="center"
    :close-on-press-escape="closeOnPressEscape"
    :aria-label="ariaLabel || undefined"
    :aria-describedby="description ? descriptionId : undefined"
    :class="dialogClasses"
    :style="[attrs.style, dialogStyle]"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="restoreTriggerFocus"
  >
    <p v-if="description" :id="descriptionId" class="app-dialog__sr-only">{{ description }}</p>
    <template
      v-if="showHeader && ($slots.header || (!title && ariaLabel))"
      #header="{ titleId, titleClass }"
    >
      <div v-if="$slots.header" :id="titleId" class="app-dialog__header-content">
        <slot name="header" />
      </div>
      <span v-else :id="titleId" :class="[titleClass, 'app-dialog__sr-only']">{{ ariaLabel }}</span>
    </template>
    <el-scrollbar class="app-dialog__scrollbar">
      <div class="app-dialog__content" :style="bodyStyle"><slot /></div>
    </el-scrollbar>
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </el-dialog>
</template>

<style scoped lang="scss">
// ElDialog teleports its root without this component's scope attribute.
// Keep root selectors explicitly namespaced; slot/content styles remain scoped.
:global(.el-dialog.app-dialog) {
  display: flex;
  flex-direction: column;
  max-height: 90vh;
}
:global(.app-dialog > .el-dialog__header),
:global(.app-dialog > .el-dialog__footer) {
  flex-shrink: 0;
}
:global(.app-dialog > .el-dialog__body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}
:global(.app-dialog--header-hidden > .el-dialog__header) {
  display: none;
}
:global(.app-dialog--custom-header-padding > .el-dialog__header) {
  padding: var(--app-dialog-header-padding);
}
:global(.app-dialog--custom-footer-padding > .el-dialog__footer) {
  padding: var(--app-dialog-footer-padding);
}
:global(.app-dialog > .el-dialog__footer) {
  border-top: 1px solid var(--el-border-color-lighter);
}

.app-dialog {
  &__content,
  &__header-content,
  &__scrollbar {
    width: 100%;
  }
  &__scrollbar {
    display: flex;
    flex-direction: column;
    height: auto;
    min-height: 0;
  }
  &__scrollbar :deep(.el-scrollbar__wrap) {
    flex: 1 1 auto;
    min-height: 0;
    height: auto;
    overflow-x: hidden;
  }
  &__sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
}
</style>
