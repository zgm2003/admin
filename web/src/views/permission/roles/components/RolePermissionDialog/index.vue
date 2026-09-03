<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getRolePermissions, updateRolePermissions } from '@/api/permission/role'
import type { RoleListItem, RolePermissionsResponse } from '@/api/permission/role'
import { AppDialog } from '@/components/AppDialog'
import { YesNo } from '@/enums/yes-no'
import {
  buildRolePermissionMatrix,
  diffMenuIDs,
  expandDirectMenuIDs,
  getRoleMatrixMenuIDs,
  normalizeDirectMenuIDs,
} from '@/views/permission/roles/role-permission-matrix'
import type {
  RoleMatrixPlatform,
  RolePermissionDiff,
} from '@/views/permission/roles/role-permission-matrix'
import RolePermissionDiffDialog from '@/views/permission/roles/components/RolePermissionDiffDialog/index.vue'
import RolePermissionMatrix from '@/views/permission/roles/components/RolePermissionMatrix/index.vue'

const props = defineProps<{ role: RoleListItem | null }>()
const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const data = ref<RolePermissionsResponse | null>(null)
const originalEffectiveMenuIDs = ref<number[]>([])
const selectedEffectiveMenuIDs = ref<number[]>([])
const activePlatformID = ref<number | null>(null)
const diffVisible = ref(false)
const diff = ref<RolePermissionDiff>({ added: [], removed: [] })

const platforms = computed<RoleMatrixPlatform[]>(() =>
  data.value === null ? [] : buildRolePermissionMatrix(data.value.platforms),
)
const groups = computed(
  () =>
    platforms.value.find((platform) => platform.platformId === activePlatformID.value)?.groups ??
    [],
)
const labelMap = computed(() => {
  const labels = new Map<number, string>()
  for (const platform of platforms.value) {
    for (const group of platform.groups) {
      for (const row of group.rows) {
        labels.set(row.pageId, `${platform.platformName} · ${row.pageName} · ${row.pageCode}`)
        for (const action of row.actions) {
          labels.set(action.id, `${platform.platformName} · ${action.name} · ${action.code}`)
        }
      }
    }
  }
  return labels
})
const addedLabels = computed(() => permissionLabels(diff.value.added))
const removedLabels = computed(() => permissionLabels(diff.value.removed))

watch(visible, (isVisible) => {
  if (isVisible) void loadPermissions()
})

function errorMessage(cause: unknown): string {
  return cause instanceof Error && cause.message !== '' ? cause.message : t('role.mutationFailed')
}

async function loadPermissions(): Promise<void> {
  const role = props.role
  if (role === null) return
  diffVisible.value = false
  loading.value = true
  error.value = ''
  data.value = null
  diff.value = { added: [], removed: [] }
  originalEffectiveMenuIDs.value = []
  selectedEffectiveMenuIDs.value = []
  try {
    const result = await getRolePermissions(role.id)
    const matrixPlatforms = buildRolePermissionMatrix(result.platforms)
    const allGroups = matrixPlatforms.flatMap((platform) => platform.groups)
    const effectiveMenuIDs = expandDirectMenuIDs(allGroups, result.menuIds)
    data.value = result
    activePlatformID.value = matrixPlatforms[0]?.platformId ?? null
    originalEffectiveMenuIDs.value = effectiveMenuIDs
    selectedEffectiveMenuIDs.value = [...effectiveMenuIDs]
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  } finally {
    loading.value = false
  }
}

function selectAll(): void {
  const selected = new Set(selectedEffectiveMenuIDs.value)
  for (const menuID of getRoleMatrixMenuIDs(groups.value)) selected.add(menuID)
  selectedEffectiveMenuIDs.value = [...selected].sort((left, right) => left - right)
}

function clear(): void {
  const currentIDs = new Set(getRoleMatrixMenuIDs(groups.value))
  selectedEffectiveMenuIDs.value = selectedEffectiveMenuIDs.value.filter(
    (menuID) => !currentIDs.has(menuID),
  )
}

function permissionLabels(menuIDs: readonly number[]): string[] {
  return menuIDs.map((menuID) => {
    const label = labelMap.value.get(menuID)
    if (label === undefined) throw new Error(`permission menu ${menuID} has no display label`)
    return label
  })
}

function prepareSave(): void {
  if (data.value === null || saving.value) return
  error.value = ''
  const nextDiff = diffMenuIDs(originalEffectiveMenuIDs.value, selectedEffectiveMenuIDs.value)
  if (nextDiff.added.length === 0 && nextDiff.removed.length === 0) {
    visible.value = false
    return
  }
  diff.value = nextDiff
  diffVisible.value = true
}

async function save(): Promise<void> {
  if (data.value === null || saving.value) return
  saving.value = true
  error.value = ''
  try {
    const allGroups = platforms.value.flatMap((platform) => platform.groups)
    await updateRolePermissions(data.value.role.id, {
      menuIds: normalizeDirectMenuIDs(allGroups, selectedEffectiveMenuIDs.value),
    })
    diffVisible.value = false
    visible.value = false
    emit('saved')
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="visible"
    class="role-permission-dialog"
    width="min(1040px, 94vw)"
    height="min(62vh, 620px)"
    append-to-body
  >
    <template #header>
      <strong>
        {{ t('role.permission.title') }}
        <template v-if="data"> · {{ data.role.name }} ({{ data.role.code }}) </template>
      </strong>
    </template>

    <div class="permission-scroll">
      <div v-if="loading">{{ t('role.permission.loading') }}</div>
      <template v-else-if="data">
        <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
        <el-tabs
          v-model="activePlatformID"
          data-testid="role-permission-platform-tabs"
          class="role-permission-platform-tabs"
        >
          <el-tab-pane
            v-for="platform in platforms"
            :key="platform.platformId"
            :name="platform.platformId"
          >
            <template #label>
              <span class="role-permission-platform-tab">
                <span>{{ platform.platformName }}</span>
                <code>{{ platform.platformCode }}</code>
                <el-tag
                  v-if="platform.platformIsEnabled === YesNo.No"
                  size="small"
                  type="info"
                  effect="plain"
                >
                  {{ t('role.permission.disabled') }}
                </el-tag>
              </span>
            </template>
          </el-tab-pane>
        </el-tabs>
        <el-space class="permission-toolbar" wrap :size="8">
          <el-button @click="selectAll">{{ t('role.permission.selectAll') }}</el-button>
          <el-button @click="clear">{{ t('role.permission.clear') }}</el-button>
        </el-space>
        <RolePermissionMatrix v-model="selectedEffectiveMenuIDs" :groups="groups" />
      </template>
      <el-alert v-else-if="error" :title="error" type="error" show-icon>
        <el-button text @click="loadPermissions">{{ t('role.retry') }}</el-button>
      </el-alert>
      <div v-else>{{ t('role.permission.empty') }}</div>
    </div>

    <template #footer>
      <el-button @click="visible = false">{{ t('role.form.cancel') }}</el-button>
      <el-button type="primary" :disabled="data === null || saving" @click="prepareSave">
        {{ t('role.permission.save') }}
      </el-button>
    </template>
  </AppDialog>

  <RolePermissionDiffDialog
    v-model="diffVisible"
    :added-labels="addedLabels"
    :removed-labels="removedLabels"
    :saving="saving"
    :error="error"
    @confirm="save"
  />
</template>

<style scoped src="./RolePermissionDialog.css"></style>
