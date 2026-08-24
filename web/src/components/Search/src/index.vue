<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import type { SearchDateRange, SearchField, SearchFormModel, SearchFormValue } from './types'

const props = withDefaults(defineProps<{
  modelValue: SearchFormModel
  fields: SearchField[]
  collapseCount?: number
  queryLabel?: string
  resetLabel?: string
  expandLabel?: string
  collapseLabel?: string
  queryTestId?: string
  resetTestId?: string
}>(), {
  collapseCount: 2,
  queryLabel: '查询',
  resetLabel: '重置',
  expandLabel: '展开',
  collapseLabel: '收起',
  queryTestId: undefined,
  resetTestId: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: SearchFormModel]
  query: [value: SearchFormModel]
  reset: [value: SearchFormModel]
}>()

const form = reactive<SearchFormModel>({ ...props.modelValue })
const collapsed = ref(false)

watch(() => props.modelValue, (value) => {
  for (const key of Object.keys(form)) {
    if (!Object.prototype.hasOwnProperty.call(value, key)) delete form[key]
  }
  Object.assign(form, value)
}, { deep: true })

const visibleFields = computed(() => {
  const count = Math.max(1, Math.floor(props.collapseCount))
  return collapsed.value ? props.fields.slice(0, count) : props.fields
})
const showToggle = computed(() => props.fields.length > Math.max(1, Math.floor(props.collapseCount)))

function resolveWidth(width: string | number | undefined): string {
  return typeof width === 'string' ? width : `${width ?? 180}px`
}

function inputValue(key: string): string | number | null | undefined {
  const value = form[key]
  if (typeof value === 'string' || typeof value === 'number' || value === null || value === undefined) return value
  throw new Error(`Search input field ${key} must be string or number`)
}

function dateRangeValue(key: string): SearchDateRange {
  const value = form[key]
  if (Array.isArray(value) && (value.length === 0 || (value.length === 2 && value.every((item) => typeof item === 'string')))) {
    return value as SearchDateRange
  }
  throw new Error(`Search date range field ${key} must be an empty or two-item string array`)
}

function normalizeValue(value: unknown, key: string): SearchFormValue {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || value === null || value === undefined) return value
  if (Array.isArray(value) && (value.length === 0 || (value.length === 2 && value.every((item) => typeof item === 'string')))) return value as SearchDateRange
  throw new Error(`Search field ${key} received an unsupported value`)
}

function setSearchValue(key: string, value: unknown): void {
  form[key] = normalizeValue(value, key)
  emit('update:modelValue', { ...form })
}

function emitForm(event: 'query' | 'reset'): void {
  const value = { ...form }
  emit('update:modelValue', value)
  if (event === 'query') emit('query', value)
  else emit('reset', value)
}

function reset(): void {
  for (const key of Object.keys(form)) form[key] = undefined
  emitForm('reset')
}
</script>

<template>
  <el-form class="search-form" :inline="true" :model="form" @submit.prevent="emitForm('query')">
    <template v-for="field in visibleFields" :key="field.key">
      <el-form-item :label="field.label" :prop="field.key">
        <el-input
          v-if="field.type === 'input'"
          :model-value="inputValue(field.key)"
          :placeholder="field.placeholder"
          :disabled="field.disabled"
          :clearable="field.clearable ?? true"
          :data-testid="field.testId"
          :style="{ width: resolveWidth(field.width) }"
          @update:model-value="setSearchValue(field.key, $event)"
        />
        <el-date-picker
          v-else-if="field.type === 'date-range'"
          :model-value="dateRangeValue(field.key)"
          type="datetimerange"
          :value-format="field.valueFormat"
          :range-separator="field.rangeSeparator"
          :start-placeholder="field.placeholder"
          :end-placeholder="field.placeholder"
          :clearable="field.clearable ?? true"
          :data-testid="field.testId"
          :style="{ width: resolveWidth(field.width) }"
          @update:model-value="setSearchValue(field.key, $event)"
        />
        <el-select-v2
          v-else
          :model-value="form[field.key]"
          :options="field.options"
          :placeholder="field.placeholder"
          :disabled="field.disabled"
          :clearable="field.clearable ?? true"
          filterable
          :data-testid="field.testId"
          :style="{ width: resolveWidth(field.width) }"
          @update:model-value="setSearchValue(field.key, $event)"
        />
      </el-form-item>
    </template>
    <el-form-item>
      <el-button type="primary" :data-testid="queryTestId" @click="emitForm('query')">{{ queryLabel }}</el-button>
      <el-button :data-testid="resetTestId" @click="reset">{{ resetLabel }}</el-button>
      <el-button
        v-if="showToggle"
        text
        :aria-expanded="!collapsed"
        @click="collapsed = !collapsed"
      >
        <el-icon>
          <component :is="collapsed ? ArrowDown : ArrowUp" />
        </el-icon>
        {{ collapsed ? expandLabel : collapseLabel }}
      </el-button>
    </el-form-item>
  </el-form>
</template>

<style scoped>
.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
}

.search-form :deep(.el-form-item) {
  margin-bottom: 12px;
}
</style>
