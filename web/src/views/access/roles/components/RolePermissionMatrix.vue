<script setup lang="ts">
import { computed, shallowRef } from "vue";
import { useI18n } from "vue-i18n";

import { YesNo } from "../../../../enums/yes-no";
import type {
  RoleMatrixGroup,
  RoleMatrixRow,
  RoleMatrixSelectionState,
} from "../role-permission-matrix";
import {
  getRoleMatrixGroupMenuIDs,
  getRoleMatrixSelectionState,
  toggleMatrixAction,
  toggleMatrixGroup,
  toggleMatrixPage,
} from "../role-permission-matrix";

const selectedMenuIDs = defineModel<number[]>({ required: true });

const props = defineProps<{
  groups: RoleMatrixGroup[];
}>();

interface GroupRuntimeState {
  selection: RoleMatrixSelectionState;
  pageTotal: number;
  pageSelected: number;
  actionTotal: number;
  actionSelected: number;
}

const { t } = useI18n();
const selectedMenuIDSet = computed(() => new Set(selectedMenuIDs.value));
const collapsedGroupIDs = shallowRef(new Set<number>());

const groupRuntimeStateMap = computed(() => {
  const states = new Map<number, GroupRuntimeState>();
  const selected = selectedMenuIDSet.value;

  for (const group of props.groups) {
    const pageIDs = group.rows.map((row) => row.pageId);
    const actionIDs = group.rows.flatMap((row) =>
      row.actions.map((action) => action.id),
    );
    states.set(group.groupId, {
      selection: getRoleMatrixSelectionState(
        getRoleMatrixGroupMenuIDs(group),
        selected,
      ),
      pageTotal: pageIDs.length,
      pageSelected: pageIDs.filter((menuID) => selected.has(menuID)).length,
      actionTotal: actionIDs.length,
      actionSelected: actionIDs.filter((menuID) => selected.has(menuID)).length,
    });
  }

  return states;
});

function groupRuntimeState(group: RoleMatrixGroup): GroupRuntimeState {
  const state = groupRuntimeStateMap.value.get(group.groupId);
  if (state === undefined) {
    throw new Error(`permission group ${group.groupId} has no runtime state`);
  }
  return state;
}

function isChecked(menuID: number): boolean {
  return selectedMenuIDSet.value.has(menuID);
}

function selectedActionCount(row: RoleMatrixRow): number {
  return row.actions.filter((action) => selectedMenuIDSet.value.has(action.id))
    .length;
}

function setPageChecked(row: RoleMatrixRow, checked: boolean): void {
  selectedMenuIDs.value = toggleMatrixPage(selectedMenuIDs.value, row, checked);
}

function setActionChecked(
  row: RoleMatrixRow,
  actionID: number,
  checked: boolean,
): void {
  selectedMenuIDs.value = toggleMatrixAction(
    selectedMenuIDs.value,
    row,
    actionID,
    checked,
  );
}

function setGroupChecked(group: RoleMatrixGroup, checked: boolean): void {
  selectedMenuIDs.value = toggleMatrixGroup(
    selectedMenuIDs.value,
    group,
    checked,
  );
}

function isGroupCollapsed(group: RoleMatrixGroup): boolean {
  return collapsedGroupIDs.value.has(group.groupId);
}

function toggleGroupCollapse(group: RoleMatrixGroup): void {
  const next = new Set(collapsedGroupIDs.value);
  if (next.has(group.groupId)) {
    next.delete(group.groupId);
  } else {
    next.add(group.groupId);
  }
  collapsedGroupIDs.value = next;
}
</script>

<template>
  <el-empty
    v-if="groups.length === 0"
    :description="t('role.permission.empty')"
  />
  <div v-else class="role-permission-matrix">
    <section
      v-for="group in groups"
      :key="group.groupId"
      class="role-permission-matrix__group"
    >
      <div class="role-permission-matrix__group-header">
        <div class="role-permission-matrix__group-main">
          <el-checkbox
            class="role-permission-matrix__group-title"
            :model-value="groupRuntimeState(group).selection.checked"
            :indeterminate="groupRuntimeState(group).selection.indeterminate"
            @update:model-value="
              (value: unknown) => setGroupChecked(group, Boolean(value))
            "
          >
            {{ group.groupName }} · {{ group.groupCode }}
            <el-tag
              v-if="group.groupIsEnabled === YesNo.No"
              size="small"
              type="danger"
            >
              {{ t("role.permission.disabled") }}
            </el-tag>
          </el-checkbox>
          <div class="role-permission-matrix__group-meta">
            <span>
              {{ t("role.permission.selected") }}
              {{ groupRuntimeState(group).selection.selected }}/{{
                groupRuntimeState(group).selection.total
              }}
            </span>
            <span>
              {{ t("role.permission.pages") }}
              {{ groupRuntimeState(group).pageSelected }}/{{
                groupRuntimeState(group).pageTotal
              }}
            </span>
            <span>
              {{ t("role.permission.actions") }}
              {{ groupRuntimeState(group).actionSelected }}/{{
                groupRuntimeState(group).actionTotal
              }}
            </span>
          </div>
        </div>
        <el-space>
          <el-button size="small" text @click="toggleGroupCollapse(group)">
            {{
              t(
                isGroupCollapsed(group)
                  ? "role.permission.expand"
                  : "role.permission.collapse",
              )
            }}
          </el-button>
          <el-button
            size="small"
            text
            type="primary"
            @click="setGroupChecked(group, true)"
          >
            {{ t("role.permission.selectAll") }}
          </el-button>
          <el-button size="small" text @click="setGroupChecked(group, false)">
            {{ t("role.permission.clear") }}
          </el-button>
        </el-space>
      </div>

      <div
        v-if="!isGroupCollapsed(group)"
        class="role-permission-matrix__table-scroll"
      >
        <el-table
          :data="group.rows"
          border
          row-key="pageId"
          class="role-permission-matrix__table"
        >
          <el-table-column :label="t('role.permission.page')" min-width="300">
            <template #default="{ row }">
              <el-checkbox
                class="role-permission-matrix__page-access"
                :model-value="isChecked(row.pageId)"
                @update:model-value="
                  (value: unknown) => setPageChecked(row, Boolean(value))
                "
              >
                <span class="role-permission-matrix__permission-copy">
                  <strong>{{ row.pageName }}</strong>
                  <span>{{ row.pageCode }}</span>
                  <span class="role-permission-matrix__page-meta">
                    {{ t("role.permission.actions") }}
                    {{ selectedActionCount(row) }}/{{ row.actions.length }}
                  </span>
                  <el-tag
                    v-if="row.pageIsEnabled === YesNo.No"
                    size="small"
                    type="danger"
                  >
                    {{ t("role.permission.disabled") }}
                  </el-tag>
                </span>
              </el-checkbox>
            </template>
          </el-table-column>
          <el-table-column :label="t('role.permission.action')" min-width="460">
            <template #default="{ row }">
              <el-space
                v-if="row.actions.length > 0"
                wrap
                class="role-permission-matrix__actions"
              >
                <el-checkbox
                  v-for="action in row.actions"
                  :key="action.id"
                  class="role-permission-matrix__action"
                  :model-value="isChecked(action.id)"
                  @update:model-value="
                    (value: unknown) =>
                      setActionChecked(row, action.id, Boolean(value))
                  "
                >
                  <span class="role-permission-matrix__permission-copy">
                    <strong>{{ action.name }}</strong>
                    <span>{{ action.code }}</span>
                    <el-tag
                      v-if="action.isEnabled === YesNo.No"
                      size="small"
                      type="danger"
                    >
                      {{ t("role.permission.disabled") }}
                    </el-tag>
                  </span>
                </el-checkbox>
              </el-space>
              <span v-else class="role-permission-matrix__no-actions">
                {{ t("role.permission.noActions") }}
              </span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>
  </div>
</template>

<style scoped src="./RolePermissionMatrix.css"></style>
