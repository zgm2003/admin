<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppDialog } from '../../AppDialog'
import { DIcon } from '../../DIcon'
import type { IconSelectIcon } from './types'

const props = withDefaults(defineProps<{
  modelValue: boolean
  icons?: IconSelectIcon[]
  title?: string
  emptyText?: string
}>(), {
  icons: () => [
    { name: 'Folder', label: 'Folder' },
    { name: 'Menu', label: 'Menu' },
    { name: 'Setting', label: 'Setting' },
    { name: 'User', label: 'User' },
    { name: 'UserFilled', label: 'UserFilled' },
    { name: 'Key', label: 'Key' },
    { name: 'List', label: 'List' },
    { name: 'Cpu', label: 'Cpu' },
    { name: 'mdi:home', label: 'mdi:home' },
    { name: 'mdi:settings', label: 'mdi:settings' },
    { name: 'mdi:account', label: 'mdi:account' },
    { name: 'mdi:shield', label: 'mdi:shield' },
    { name: 'lucide:database', label: 'lucide:database' },
    { name: 'lucide:users', label: 'lucide:users' },
  ],
  title: '选择图标',
  emptyText: '暂无匹配图标',
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select-icon': [value: string]
}>()

const search = ref('')
const selected = ref('')
const filteredIcons = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  if (keyword === '') return props.icons
  return props.icons.filter((icon) => `${icon.name} ${icon.label}`.toLowerCase().includes(keyword))
})

function selectIcon(name: string): void {
  selected.value = name
  emit('select-icon', name)
  emit('update:modelValue', false)
}
</script>

<template>
  <AppDialog
    :model-value="modelValue"
    :title="title"
    width="min(760px, 94vw)"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-input v-model="search" clearable placeholder="搜索图标" />
    <div v-if="filteredIcons.length > 0" class="icon-select-grid">
      <button
        v-for="icon in filteredIcons"
        :key="icon.name"
        type="button"
        class="icon-select-item"
        :class="{ 'is-selected': selected === icon.name }"
        :aria-label="icon.label"
        @click="selectIcon(icon.name)"
      >
        <DIcon :icon="icon.name" :size="24" />
        <span>{{ icon.label }}</span>
      </button>
    </div>
    <el-empty v-else :description="emptyText" />
  </AppDialog>
</template>

<style scoped>
.icon-select-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 8px;
  max-height: 420px;
  margin-top: 12px;
  overflow-y: auto;
}

.icon-select-item {
  display: flex;
  min-height: 76px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color);
  cursor: pointer;
}

.icon-select-item:hover,
.icon-select-item.is-selected {
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
}
</style>
