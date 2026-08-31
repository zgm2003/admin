<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppDialog } from '../../AppDialog'
import { AppDIcon } from '../../AppDIcon'
import { menuIcons, type MenuIconName } from '../../../icons/menu-icons'
import type { IconSelectIcon } from './types'

const props = withDefaults(defineProps<{
  modelValue: boolean
  icons?: readonly IconSelectIcon[]
  title?: string
  emptyText?: string
}>(), {
  icons: () => Object.entries(menuIcons).map(([name, value]) => ({ name: name as MenuIconName, label: value.label })),
  title: '选择图标',
  emptyText: '暂无匹配图标',
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select-icon': [value: MenuIconName]
}>()

const search = ref('')
const selected = ref<MenuIconName | null>(null)
const filteredIcons = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  if (keyword === '') return props.icons
  return props.icons.filter((icon) => `${icon.name} ${icon.label}`.toLocaleLowerCase().includes(keyword))
})

function selectIcon(name: MenuIconName): void {
  selected.value = name
  emit('select-icon', name)
  emit('update:modelValue', false)
}
</script>

<template>
  <AppDialog :model-value="modelValue" :title="title" width="min(760px, 94vw)" @update:model-value="emit('update:modelValue', $event)">
    <el-input v-model="search" clearable placeholder="搜索图标" />
    <div v-if="filteredIcons.length > 0" class="icon-select-grid">
      <button v-for="icon in filteredIcons" :key="icon.name" type="button" class="icon-select-item" :class="{ 'is-selected': selected === icon.name }" :aria-label="icon.label" @click="selectIcon(icon.name)">
        <AppDIcon :icon="icon.name" :size="24" :title="icon.label" />
        <span>{{ icon.label }}</span>
      </button>
    </div>
    <el-empty v-else :description="emptyText" />
  </AppDialog>
</template>

<style scoped>
.icon-select-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(72px, 1fr)); gap: 10px; max-height: 420px; margin-top: 12px; overflow-y: auto; scrollbar-width: none; -ms-overflow-style: none; }
.icon-select-grid::-webkit-scrollbar { display: none; }
.icon-select-item { display: flex; min-height: 84px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; border: 1px solid var(--el-border-color); border-radius: 4px; color: var(--el-text-color-primary); background: var(--el-bg-color); cursor: pointer; }
.icon-select-item:hover, .icon-select-item.is-selected { border-color: var(--el-color-primary); color: var(--el-color-primary); }
</style>
