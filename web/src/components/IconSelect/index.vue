<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AppDialog } from '@/components/AppDialog'
import { AppDIcon } from '@/components/AppDIcon'
import { menuIcons, type MenuIconName } from '@/icons/menu-icons'
import type { IconSelectIcon } from './types'

defineOptions({ name: 'IconSelect' })

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    icons?: readonly IconSelectIcon[]
    title?: string
    emptyText?: string
  }>(),
  {
    icons: () =>
      Object.entries(menuIcons).map(([name, value]) => ({
        name: name as MenuIconName,
        label: value.label,
      })),
    title: '选择图标',
    emptyText: '暂无匹配图标',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select-icon': [value: MenuIconName]
}>()

const search = ref('')
const selected = ref<MenuIconName | null>(null)
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) search.value = ''
  },
)
const filteredIcons = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  if (keyword === '') return props.icons
  return props.icons.filter((icon) =>
    `${icon.name} ${icon.label}`.toLocaleLowerCase().includes(keyword),
  )
})

function selectIcon(name: MenuIconName): void {
  selected.value = name
  emit('select-icon', name)
  emit('update:modelValue', false)
}

function handleItemKeydown(event: KeyboardEvent, name: MenuIconName): void {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    selectIcon(name)
  }
}
</script>

<template>
  <AppDialog
    :model-value="modelValue"
    :title="title"
    width="min(760px, 94vw)"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="icon-select-toolbar">
      <el-input v-model="search" clearable placeholder="搜索图标" />
      <el-text type="info" size="small">{{ filteredIcons.length }} 个图标</el-text>
    </div>
    <el-scrollbar v-if="filteredIcons.length > 0" height="500px" class="icon-select-scroll">
      <div class="icon-select-grid">
        <div
          v-for="icon in filteredIcons"
          :key="icon.name"
          class="icon-select-item"
          :class="{ 'is-selected': selected === icon.name }"
          role="button"
          tabindex="0"
          :aria-label="icon.label"
          @click="selectIcon(icon.name)"
          @keydown="handleItemKeydown($event, icon.name)"
        >
          <AppDIcon :icon="icon.name" :size="28" :title="icon.label" />
          <span>{{ icon.label }}</span>
        </div>
      </div>
    </el-scrollbar>
    <el-empty v-else :description="emptyText" :image-size="100" />
  </AppDialog>
</template>

<style scoped>
.icon-select-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.icon-select-toolbar .el-input {
  flex: 1;
}
.icon-select-scroll {
  padding: 2px;
}
.icon-select-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
  gap: 10px;
}
.icon-select-item {
  display: flex;
  width: 100%;
  min-height: 84px;
  padding: 10px 6px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color);
  cursor: pointer;
  user-select: none;
  transition:
    border-color 0.2s,
    color 0.2s,
    background-color 0.2s;
}
.icon-select-item span {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.icon-select-item:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.icon-select-item:hover,
.icon-select-item.is-selected {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}
.icon-select-item.is-selected {
  box-shadow: 0 0 0 2px var(--el-color-primary-light-8);
}
</style>
