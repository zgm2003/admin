import { isMenuIconKey, hasRouteViewKey } from '../access/protocol'
import type { MenuIconKey } from '../access/menu-icons'
import { isMenuTitleKey, type MenuTitleKey } from '../access/menu-title-keys'
import { ProtocolError } from '../types/http'

export type MenuType = 'directory' | 'page'

export interface AccessMenuNode {
  code: string
  menuType: MenuType
  path: string | null
  viewKey: string | null
  titleKey: MenuTitleKey
  icon: MenuIconKey | null
  children: AccessMenuNode[]
}

export interface AccessSnapshot {
  roleCodes: string[]
  menuTree: AccessMenuNode[]
  permissionCodes: string[]
}

interface ParseState {
  codes: Set<string>
  pagePaths: Set<string>
}

export function parseAccessSnapshot(value: unknown): AccessSnapshot {
  const record = closedRecord(value, ['menuTree', 'permissionCodes', 'roleCodes'], 'access snapshot')
  const roleCodes = parseSortedUniqueStrings(record.roleCodes, 'access snapshot roleCodes')
  const permissionCodes = parseSortedUniqueStrings(record.permissionCodes, 'access snapshot permissionCodes')
  if (!Array.isArray(record.menuTree)) {
    throw new ProtocolError('access snapshot menuTree must be an array')
  }

  const state: ParseState = { codes: new Set<string>(), pagePaths: new Set<string>() }
  const menuTree = record.menuTree.map((node, index) => parseMenuNode(node, `access snapshot menuTree[${index}]`, state))
  return { roleCodes, menuTree, permissionCodes }
}

function parseMenuNode(value: unknown, label: string, state: ParseState): AccessMenuNode {
  const record = closedRecord(value, ['children', 'code', 'icon', 'menuType', 'path', 'titleKey', 'viewKey'], label)
  const code = nonEmptyString(record.code, `${label} code`)
  if (state.codes.has(code)) {
    throw new ProtocolError(`${label} duplicates menu code ${code}`)
  }
  state.codes.add(code)

  if (record.menuType !== 'directory' && record.menuType !== 'page') {
    throw new ProtocolError(`${label} menuType must be directory or page`)
  }
  const menuType = record.menuType

	if (typeof record.titleKey !== 'string' || !isMenuTitleKey(record.titleKey)) {
    throw new ProtocolError(`${label} titleKey is not registered`)
  }
  const titleKey = record.titleKey

  if (record.icon !== null && (typeof record.icon !== 'string' || !isMenuIconKey(record.icon))) {
    throw new ProtocolError(`${label} icon is not registered`)
  }
  const icon = record.icon

  if (!Array.isArray(record.children)) {
    throw new ProtocolError(`${label} children must be an array`)
  }

  if (menuType === 'directory') {
		if (record.path !== null || record.viewKey !== null) {
			throw new ProtocolError(`${label} directory path and viewKey must be null`)
    }
    const children = record.children.map((child, index) => parseMenuNode(child, `${label}.children[${index}]`, state))
		return { code, menuType, path: null, viewKey: null, titleKey, icon, children }
  }

  const path = nonEmptyString(record.path, `${label} page path`)
  const viewKey = nonEmptyString(record.viewKey, `${label} page viewKey`)
  if (!hasRouteViewKey(viewKey)) {
    throw new ProtocolError(`${label} page viewKey is not registered`)
  }
  if (record.children.length !== 0) {
    throw new ProtocolError(`${label} page children must be empty`)
  }
  if (state.pagePaths.has(path)) {
    throw new ProtocolError(`${label} duplicates page path ${path}`)
  }
  state.pagePaths.add(path)
  return { code, menuType, path, viewKey, titleKey, icon, children: [] }
}

function parseSortedUniqueStrings(value: unknown, label: string): string[] {
  if (!Array.isArray(value)) {
    throw new ProtocolError(`${label} must be an array`)
  }
  const result: string[] = []
  for (let index = 0; index < value.length; index += 1) {
    const item = nonEmptyString(value[index], `${label}[${index}]`)
    if (index > 0 && result[index - 1] >= item) {
      throw new ProtocolError(`${label} must be sorted and deduplicated`)
    }
    result.push(item)
  }
  return result
}

function nonEmptyString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) {
    throw new ProtocolError(`${label} must be a non-empty trimmed string`)
  }
  return value
}

function closedRecord(value: unknown, expectedKeys: string[], label: string): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new ProtocolError(`${label} must be an object`)
  }
  const actualKeys = Object.keys(value).sort()
  const sortedExpectedKeys = [...expectedKeys].sort()
  if (actualKeys.length !== sortedExpectedKeys.length || actualKeys.some((key, index) => key !== sortedExpectedKeys[index])) {
    throw new ProtocolError(`${label} contains unexpected or missing fields`)
  }
  return value
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
