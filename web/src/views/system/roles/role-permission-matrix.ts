import type { RolePermissionTreeNode } from '../../../api/role.contract'
import type { YesNo } from '../../../enums/yes-no'

export interface RoleMatrixAction {
  id: number
  code: string
  i18nKey: string
  isEnabled: YesNo
}

export interface RoleMatrixRow {
  pageId: number
  pageCode: string
  pageI18nKey: string
  pageIsEnabled: YesNo
  actions: RoleMatrixAction[]
}

export interface RoleMatrixGroup {
  groupId: number
  groupCode: string
  groupI18nKey: string
  groupIsEnabled: YesNo
  rows: RoleMatrixRow[]
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
  nodes: readonly RolePermissionTreeNode[],
): RoleMatrixGroup[] {
  return nodes.map((node) => {
    if (node.menuType !== 'directory') {
      throw new Error(`root permission menu ${node.id} must be a directory`)
    }

    const rows: RoleMatrixRow[] = []
    collectRows(node.children, rows)

    return {
      groupId: node.id,
      groupCode: node.code,
      groupI18nKey: node.i18nKey,
      groupIsEnabled: node.isEnabled,
      rows,
    }
  }).filter((group) => group.rows.length > 0)
}

function collectRows(
  nodes: readonly RolePermissionTreeNode[],
  rows: RoleMatrixRow[],
): void {
  for (const node of nodes) {
    if (node.menuType === 'directory') {
      collectRows(node.children, rows)
      continue
    }
    if (node.menuType !== 'page') {
      throw new Error(`permission menu ${node.id} must be grouped under a page`)
    }

    rows.push({
      pageId: node.id,
      pageCode: node.code,
      pageI18nKey: node.i18nKey,
      pageIsEnabled: node.isEnabled,
      actions: node.children.map((action) => {
        if (action.menuType !== 'action') {
          throw new Error(`page permission menu ${node.id} contains a non-action child`)
        }
        return {
          id: action.id,
          code: action.code,
          i18nKey: action.i18nKey,
          isEnabled: action.isEnabled,
        }
      }),
    })
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
