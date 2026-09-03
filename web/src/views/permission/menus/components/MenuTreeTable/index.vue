<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CirclePlus, Refresh } from '@element-plus/icons-vue'

import type { ManagedMenuNode, ManagedMenuType } from '@/api/permission/menu'
import { AppDIcon } from '@/components/AppDIcon'
import { YesNo } from '@/enums/yes-no'
import { filterManagedMenuTree } from '@/views/permission/menus/filter-menu-tree'
import { flattenWithChildren, menuRowKey } from '@/views/permission/menus/menu-tree'

const props = defineProps<{
  menus: readonly ManagedMenuNode[]
  loading: boolean
  showTable: boolean
  canCreate: boolean
  canUpdate: boolean
  canDelete: boolean
  canRebuildAccessCache: boolean
  activePlatformAvailable: boolean
  rebuildingAccessCache: boolean
}>()
const emit = defineEmits<{
  'create-root': []
  'create-child': [node: ManagedMenuNode]
  edit: [node: ManagedMenuNode]
  status: [node: ManagedMenuNode]
  delete: [node: ManagedMenuNode]
  refresh: []
  'rebuild-access-cache': []
}>()

const { t } = useI18n()
const keyword = ref('')
const expandedIDs = ref<Set<string>>(new Set())
const expansionBeforeSearch = ref<Set<string> | null>(null)
const displayedMenus = computed(() => filterManagedMenuTree(props.menus, keyword.value))
const expandedRowKeys = computed(() => [...expandedIDs.value])
const tableHeaderCellStyle = { background: 'var(--el-fill-color-light)' }

watch(
  () => props.menus,
  () => {
    if (keyword.value.trim() === '') collapseAll()
  },
)

function menuTypeLabel(menuType: ManagedMenuType): string {
  return t(`menu.type.${menuType}`)
}

function menuTypeTag(menuType: ManagedMenuType): 'primary' | 'success' | 'warning' {
  if (menuType === 'directory') return 'primary'
  if (menuType === 'page') return 'success'
  return 'warning'
}

function expandAll(): void {
  expandedIDs.value = new Set(
    flattenWithChildren(displayedMenus.value)
      .filter((node) => node.children.length > 0)
      .map((node) => menuRowKey(node.id)),
  )
}

function collapseAll(): void {
  expandedIDs.value = new Set()
}

function updateKeyword(value: string): void {
  const wasEmpty = keyword.value.trim() === ''
  const isEmpty = value.trim() === ''
  if (wasEmpty && !isEmpty) expansionBeforeSearch.value = new Set(expandedIDs.value)
  keyword.value = value
  if (!isEmpty) expandAll()
  else if (!wasEmpty) expandedIDs.value = expansionBeforeSearch.value ?? new Set()
}
</script>

<template>
  <div class="menu-tree-table">
    <div class="menu-tree-table__toolbar management-page__actions">
      <el-input
        data-testid="menu-search"
        :model-value="keyword"
        clearable
        :placeholder="t('menu.search.placeholder')"
        @update:model-value="updateKeyword"
      />
      <el-button data-testid="menu-expand-all" @click="expandAll">
        {{ t('menu.expandAll') }}
      </el-button>
      <el-button data-testid="menu-collapse-all" @click="collapseAll">
        {{ t('menu.collapseAll') }}
      </el-button>
      <el-button
        v-if="canCreate"
        data-testid="add-root-menu"
        type="primary"
        :icon="CirclePlus"
        :disabled="!activePlatformAvailable"
        @click="emit('create-root')"
      >
        {{ t('menu.addRoot') }}
      </el-button>
      <el-button
        data-testid="refresh-menus"
        :icon="Refresh"
        :loading="loading"
        @click="emit('refresh')"
      >
        {{ t('menu.refresh') }}
      </el-button>
      <el-button
        v-if="canRebuildAccessCache"
        data-testid="rebuild-access-cache"
        :icon="Refresh"
        :loading="rebuildingAccessCache"
        @click="emit('rebuild-access-cache')"
      >
        {{ t('menu.rebuildAccessCache') }}
      </el-button>
    </div>

    <!-- AppTable exception: Element Plus tree rows and controlled expand-row-keys are required here. -->
    <el-table
      v-if="showTable"
      v-loading="loading"
      border
      data-testid="menu-table"
      class="menu-tree-table__table"
      :data="displayedMenus"
      :expand-row-keys="expandedRowKeys"
      :header-cell-style="tableHeaderCellStyle"
      row-key="id"
      :tree-props="{ children: 'children' }"
      table-layout="fixed"
    >
      <el-table-column
        :label="t('menu.column.title')"
        min-width="190"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <span class="menu-title-cell">{{ row.name }}</span>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.type')"
        width="116"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <el-tag size="small" effect="plain" :type="menuTypeTag(row.menuType)">
            {{ menuTypeLabel(row.menuType) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column
        prop="code"
        :label="t('menu.column.code')"
        min-width="190"
        align="center"
        header-align="center"
        show-overflow-tooltip
      />

      <el-table-column
        :label="t('menu.column.remark')"
        min-width="220"
        align="center"
        header-align="center"
        show-overflow-tooltip
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <span v-if="row.remark !== null">{{ row.remark }}</span>
          <span v-else class="menu-cell-empty">-</span>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.route')"
        min-width="180"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <code v-if="row.path !== null" class="menu-route-cell">{{ row.path }}</code>
          <span v-else class="menu-cell-empty">-</span>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.componentPath')"
        min-width="190"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <code v-if="row.componentPath !== null" class="menu-route-cell">
            {{ row.componentPath }}
          </code>
          <span v-else class="menu-cell-empty">-</span>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.icon')"
        width="112"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <span v-if="row.icon !== null" class="menu-icon-cell">
            <AppDIcon :icon="row.icon" :size="24" :title="row.icon" />
          </span>
          <span v-else class="menu-cell-empty">-</span>
        </template>
      </el-table-column>

      <el-table-column
        prop="sortOrder"
        :label="t('menu.column.sortOrder')"
        width="88"
        align="center"
        header-align="center"
      />

      <el-table-column
        :label="t('menu.column.visibility')"
        width="104"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <el-tag
            :type="row.isHidden === YesNo.No ? 'success' : 'info'"
            size="small"
            effect="plain"
          >
            {{
              row.isHidden === YesNo.No ? t('menu.visibility.visible') : t('menu.visibility.hidden')
            }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.status')"
        width="104"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <span :data-menu-enabled="row.isEnabled">
            <el-tag
              :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'"
              size="small"
              effect="plain"
            >
              {{
                row.isEnabled === YesNo.Yes ? t('menu.status.enabled') : t('menu.status.disabled')
              }}
            </el-tag>
          </span>
        </template>
      </el-table-column>

      <el-table-column
        :label="t('menu.column.actions')"
        :width="280"
        fixed="right"
        align="center"
        header-align="center"
      >
        <template #default="{ row }: { row: ManagedMenuNode }">
          <el-button
            v-if="canCreate && row.menuType !== 'action'"
            :data-testid="`add-child-${row.id}`"
            text
            type="primary"
            @click="emit('create-child', row)"
          >
            {{ t('menu.addChild') }}
          </el-button>
          <el-button
            v-if="canUpdate"
            :data-testid="`edit-${row.id}`"
            text
            type="primary"
            @click="emit('edit', row)"
          >
            {{ t('menu.edit') }}
          </el-button>
          <el-button
            v-if="canUpdate"
            :data-testid="`status-${row.id}`"
            text
            type="warning"
            :disabled="row.isProtected === YesNo.Yes"
            :title="row.isProtected === YesNo.Yes ? t('menu.form.protectedHint') : undefined"
            @click="emit('status', row)"
          >
            {{ row.isEnabled === YesNo.Yes ? t('menu.disable') : t('menu.enable') }}
          </el-button>
          <el-button
            v-if="canDelete"
            :data-testid="`delete-${row.id}`"
            text
            type="danger"
            :disabled="row.isProtected === YesNo.Yes"
            :title="row.isProtected === YesNo.Yes ? t('menu.form.protectedHint') : undefined"
            @click="emit('delete', row)"
          >
            {{ t('menu.delete') }}
          </el-button>
        </template>
      </el-table-column>

      <template #empty>
        <div data-testid="menu-empty" class="menu-tree-table__empty">
          {{ t('menu.empty') }}
        </div>
      </template>
    </el-table>
  </div>
</template>

<style scoped src="./MenuTreeTable.css"></style>
