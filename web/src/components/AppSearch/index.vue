<script
  setup
  lang="ts"
  generic="TModel extends Record<string, SearchFormValue> = Record<string, SearchFormValue>"
>
import { computed, reactive, ref, watch } from 'vue'
import { ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import { ElSpace } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { SearchDateRange, SearchField, SearchFormValue } from './types'

defineOptions({ name: 'AppSearch' })

const props = withDefaults(
  defineProps<{
    modelValue: TModel
    fields: SearchField<TModel>[]
    collapseCount?: number
    queryLabel?: string
    resetLabel?: string
    expandLabel?: string
    collapseLabel?: string
    queryTestId?: string
    resetTestId?: string
  }>(),
  {
    collapseCount: 2,
    queryLabel: undefined,
    resetLabel: undefined,
    expandLabel: undefined,
    collapseLabel: undefined,
    queryTestId: undefined,
    resetTestId: undefined,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: TModel]
  query: [value: TModel]
  reset: [value: TModel]
}>()

const { t } = useI18n()
const resolvedQueryLabel = computed(() => props.queryLabel ?? t('search.query'))
const resolvedResetLabel = computed(() => props.resetLabel ?? t('search.reset'))
const resolvedExpandLabel = computed(() => props.expandLabel ?? t('search.expand'))
const resolvedCollapseLabel = computed(() => props.collapseLabel ?? t('search.collapse'))

const form = reactive<Record<string, SearchFormValue>>({ ...props.modelValue } as Record<
  string,
  SearchFormValue
>)
const collapsed = ref(false)
const validationError = ref<string | null>(null)

function isScalar(value: unknown): value is string | number | null | undefined {
  return (
    typeof value === 'string' || typeof value === 'number' || value === null || value === undefined
  )
}

function isDateRange(value: unknown): value is SearchDateRange {
  return (
    Array.isArray(value) &&
    (value.length === 0 || (value.length === 2 && value.every((item) => typeof item === 'string')))
  )
}

function validateModel(fields: readonly SearchField<TModel>[], model: TModel): string | null {
  for (const field of fields) {
    if (!Object.prototype.hasOwnProperty.call(model, field.key))
      return `missing:${String(field.key)}`
    const value = (model as Record<string, unknown>)[field.key]
    if (field.type === 'date-range' ? !isDateRange(value) : !isScalar(value))
      return `invalid:${String(field.key)}`
  }
  return null
}

validationError.value = validateModel(props.fields, props.modelValue)

watch(
  () => props.modelValue,
  (value) => {
    for (const key of Object.keys(form)) {
      if (!Object.prototype.hasOwnProperty.call(value, key)) delete form[key]
    }
    Object.assign(form, value)
    validationError.value = validateModel(props.fields, value)
  },
  { deep: true },
)

const visibleFields = computed(() => {
  const count = Math.max(1, Math.floor(props.collapseCount))
  return collapsed.value ? props.fields.slice(0, count) : props.fields
})
const showToggle = computed(
  () => props.fields.length > Math.max(1, Math.floor(props.collapseCount)),
)

function resolveWidth(width: string | number | undefined): string {
  return typeof width === 'string' ? width : `${width ?? 180}px`
}

function inputValue(key: string): string | number | null | undefined {
  const value = form[key]
  return isScalar(value) ? value : undefined
}

function dateRangeValue(key: string): SearchDateRange {
  const value = form[key]
  return isDateRange(value) ? value : []
}

function selectValue(key: string): string | number | null | undefined {
  const value = form[key]
  return isScalar(value) ? value : undefined
}

function normalizeValue(value: unknown): SearchFormValue | null {
  if (isScalar(value)) return value
  if (isDateRange(value)) return value
  return null
}

function setSearchValue(key: string, value: unknown): void {
  const normalized = normalizeValue(value)
  if (normalized === null) return
  form[key] = normalized
  emit('update:modelValue', { ...form } as TModel)
}

function emitForm(event: 'query' | 'reset'): void {
  validationError.value = validateModel(props.fields, { ...form } as TModel)
  if (validationError.value) return
  const value = { ...form }
  emit('update:modelValue', value as TModel)
  if (event === 'query') emit('query', value as TModel)
  else emit('reset', value as TModel)
}

function reset(): void {
  for (const field of props.fields) form[field.key] = field.type === 'date-range' ? [] : undefined
  emitForm('reset')
}
</script>

<template>
  <el-form class="search-form" :inline="true" :model="form" @submit.prevent="emitForm('query')">
    <div v-if="validationError" role="alert" class="search-form__error">
      {{ t('search.invalidModel') }}
    </div>
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
          :model-value="selectValue(field.key)"
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
      <el-space wrap size="small">
        <el-button type="primary" :data-testid="queryTestId" @click="emitForm('query')">{{
          resolvedQueryLabel
        }}</el-button>
        <el-button :data-testid="resetTestId" @click="reset">{{ resolvedResetLabel }}</el-button>
        <el-button
          v-if="showToggle"
          text
          :aria-expanded="!collapsed"
          @click="collapsed = !collapsed"
        >
          <el-icon>
            <component :is="collapsed ? ArrowDown : ArrowUp" />
          </el-icon>
          {{ collapsed ? resolvedExpandLabel : resolvedCollapseLabel }}
        </el-button>
      </el-space>
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
