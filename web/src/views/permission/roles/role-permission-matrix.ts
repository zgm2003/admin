import type { RolePermissionPlatform, RolePermissionTreeNode } from '@/api/permission/role'
import type { YesNo } from '@/enums/yes-no'

export interface RoleMatrixAction {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface RoleMatrixRow {
  pageId: number
  pageCode: string
  pageName: string
  pageIsEnabled: YesNo
  actions: RoleMatrixAction[]
}

export interface RoleMatrixGroup {
  groupKey: string
  groupCode: string
  groupName: string
  groupIsEnabled: YesNo
  rows: RoleMatrixRow[]
}

export interface RoleMatrixPlatform {
  platformId: number
  platformCode: string
  platformName: string
  platformIsEnabled: YesNo
  groups: RoleMatrixGroup[]
}

export interface RoleMatrixSelectionState {
  total: number
  selected: number
  checked: boolean
  indeterminate: boolean
}

export interface RolePermissionDiff {
  added: number[]
  removed: number[]
}

export function buildRolePermissionMatrix(
  platforms: readonly RolePermissionPlatform[],
): RoleMatrixPlatform[] {
  return platforms.map((platform) => ({
    platformId: platform.id,
    platformCode: platform.code,
    platformName: platform.name,
    platformIsEnabled: platform.isEnabled,
    groups: buildPlatformGroups(platform),
  }))
}

function buildPlatformGroups(platform: RolePermissionPlatform): RoleMatrixGroup[] {
  const groups: RoleMatrixGroup[] = []
  const rootPageRows: RoleMatrixRow[] = []
  for (const node of platform.menuTree) {
    if (node.menuType === 'action') {
      throw new Error(`root permission menu ${node.id} cannot be an action`)
    }
    if (node.menuType === 'page') {
      rootPageRows.push(buildRow(node))
      continue
    }
    const rows: RoleMatrixRow[] = []
    collectRows(node.children, rows)
    if (rows.length > 0) {
      groups.push({
        groupKey: `menu:${node.id}`,
        groupCode: node.code,
        groupName: node.name,
        groupIsEnabled: node.isEnabled,
        rows,
      })
    }
  }
  if (rootPageRows.length > 0) {
    groups.unshift({
      groupKey: `platform:${platform.id}`,
      groupCode: platform.code,
      groupName: platform.name,
      groupIsEnabled: platform.isEnabled,
      rows: rootPageRows,
    })
  }
  return groups
}

function collectRows(nodes: readonly RolePermissionTreeNode[], rows: RoleMatrixRow[]): void {
  for (const node of nodes) {
    if (node.menuType === 'directory') {
      collectRows(node.children, rows)
      continue
    }
    if (node.menuType !== 'page') {
      throw new Error(`permission menu ${node.id} must be grouped under a page`)
    }

    rows.push(buildRow(node))
  }
}

function buildRow(node: RolePermissionTreeNode): RoleMatrixRow {
  if (node.menuType !== 'page') {
    throw new Error(`permission menu ${node.id} is not a page`)
  }
  return {
    pageId: node.id,
    pageCode: node.code,
    pageName: node.name,
    pageIsEnabled: node.isEnabled,
    actions: node.children.map((action) => {
      if (action.menuType !== 'action' || action.children.length !== 0) {
        throw new Error(`page permission menu ${node.id} contains an invalid action child`)
      }
      return {
        id: action.id,
        code: action.code,
        name: action.name,
        isEnabled: action.isEnabled,
      }
    }),
  }
}

export function getRoleMatrixRowMenuIDs(row: RoleMatrixRow): number[] {
  return [row.pageId, ...row.actions.map((action) => action.id)]
}

export function getRoleMatrixGroupMenuIDs(group: RoleMatrixGroup): number[] {
  return sortedUnique(group.rows.flatMap(getRoleMatrixRowMenuIDs))
}

export function getRoleMatrixMenuIDs(groups: readonly RoleMatrixGroup[]): number[] {
  return sortedUnique(groups.flatMap(getRoleMatrixGroupMenuIDs))
}

export function getRoleMatrixSelectionState(
  menuIDs: readonly number[],
  selected: readonly number[] | ReadonlySet<number>,
): RoleMatrixSelectionState {
  const selectedSet = selected instanceof Set ? selected : new Set(selected)
  const selectedCount = menuIDs.reduce(
    (count, menuID) => count + (selectedSet.has(menuID) ? 1 : 0),
    0,
  )

  return {
    total: menuIDs.length,
    selected: selectedCount,
    checked: menuIDs.length > 0 && selectedCount === menuIDs.length,
    indeterminate: selectedCount > 0 && selectedCount < menuIDs.length,
  }
}

export function expandDirectMenuIDs(
  groups: readonly RoleMatrixGroup[],
  directMenuIDs: readonly number[],
): number[] {
  const rowsByMenuID = indexRows(groups)
  const expanded = new Set<number>()

  for (const menuID of directMenuIDs) {
    const row = rowsByMenuID.get(menuID)
    if (row === undefined) {
      throw new Error(`direct permission menu ${menuID} is absent from the matrix`)
    }
    expanded.add(menuID)
    if (menuID !== row.pageId) {
      expanded.add(row.pageId)
    }
  }

  return sortedUnique(expanded)
}

export function toggleMatrixPage(
  selected: readonly number[],
  row: RoleMatrixRow,
  checked: boolean,
): number[] {
  const next = new Set(selected)
  if (checked) {
    next.add(row.pageId)
  } else {
    for (const menuID of getRoleMatrixRowMenuIDs(row)) {
      next.delete(menuID)
    }
  }
  return sortedUnique(next)
}

export function toggleMatrixAction(
  selected: readonly number[],
  row: RoleMatrixRow,
  actionID: number,
  checked: boolean,
): number[] {
  if (!row.actions.some((action) => action.id === actionID)) {
    throw new Error(`action permission menu ${actionID} is absent from page ${row.pageId}`)
  }

  const next = new Set(selected)
  if (checked) {
    next.add(row.pageId)
    next.add(actionID)
  } else {
    next.delete(actionID)
  }
  return sortedUnique(next)
}

export function toggleMatrixGroup(
  selected: readonly number[],
  group: RoleMatrixGroup,
  checked: boolean,
): number[] {
  const next = new Set(selected)
  for (const menuID of getRoleMatrixGroupMenuIDs(group)) {
    if (checked) {
      next.add(menuID)
    } else {
      next.delete(menuID)
    }
  }
  return sortedUnique(next)
}

export function normalizeDirectMenuIDs(
  groups: readonly RoleMatrixGroup[],
  effectiveMenuIDs: readonly number[],
): number[] {
  const selected = new Set(effectiveMenuIDs)
  const direct = new Set<number>()

  for (const group of groups) {
    for (const row of group.rows) {
      const selectedActionIDs = row.actions
        .filter((action) => selected.has(action.id))
        .map((action) => action.id)
      if (selectedActionIDs.length > 0) {
        selectedActionIDs.forEach((actionID) => direct.add(actionID))
      } else if (selected.has(row.pageId)) {
        direct.add(row.pageId)
      }
    }
  }

  return sortedUnique(direct)
}

export function diffMenuIDs(
  before: readonly number[],
  after: readonly number[],
): RolePermissionDiff {
  const beforeSet = new Set(before)
  const afterSet = new Set(after)
  return {
    added: sortedUnique(after.filter((menuID) => !beforeSet.has(menuID))),
    removed: sortedUnique(before.filter((menuID) => !afterSet.has(menuID))),
  }
}

function indexRows(groups: readonly RoleMatrixGroup[]): Map<number, RoleMatrixRow> {
  const rowsByMenuID = new Map<number, RoleMatrixRow>()
  for (const group of groups) {
    for (const row of group.rows) {
      rowsByMenuID.set(row.pageId, row)
      row.actions.forEach((action) => rowsByMenuID.set(action.id, row))
    }
  }
  return rowsByMenuID
}

function sortedUnique(values: Iterable<number>): number[] {
  return [...new Set(values)].sort((left, right) => left - right)
}
