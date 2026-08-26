import {
  isComponentPath,
  isMenuI18nKey,
  isMenuIcon,
  isMenuPath,
  menuCodePattern,
} from './menu-fields'
import { isYesNo, type YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'

export type ManagedMenuType = 'directory' | 'page' | 'action'

export interface ManagedMenuNode {
  id: number
  parentId: number | null
  menuType: ManagedMenuType
	name: string
  code: string
	i18nKey: string | null
  path: string | null
	componentPath: string | null
	icon: string | null
  sortOrder: number
  isEnabled: YesNo
	isHidden: YesNo
	isProtected: YesNo
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}

export interface CreateMenuInput {
  parentId: number | null
  menuType: ManagedMenuType
	name: string
  code: string
	i18nKey: string | null
  path: string | null
	componentPath: string | null
	icon: string | null
  sortOrder: number
  isEnabled: YesNo
	isHidden: YesNo
}

export interface UpdateMenuInput {
  parentId: number | null
  menuType: ManagedMenuType
	name: string
	i18nKey: string | null
  path: string | null
	componentPath: string | null
	icon: string | null
  sortOrder: number
	isHidden: YesNo
}

export interface MenuIDResult {
  id: number
}

export interface MenuStatusResult {
  id: number
  isEnabled: YesNo
}

interface ParseState {
  ids: Set<number>
  codes: Set<string>
  pagePaths: Set<string>
}

const menuNodeKeys = [
  'children',
  'code',
	'componentPath',
  'createdAt',
  'icon',
  'id',
  'i18nKey',
  'isEnabled',
	'isHidden',
	'isProtected',
  'menuType',
	'name',
  'parentId',
  'path',
  'sortOrder',
  'updatedAt',
]

const rfc3339Pattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

export function parseManagedMenus(value: unknown): ManagedMenuNode[] {
  if (!Array.isArray(value)) {
    throw new ProtocolError('managed menus must be an array')
  }
  const state: ParseState = { ids: new Set<number>(), codes: new Set<string>(), pagePaths: new Set<string>() }
  const roots = value.map((item, index) => parseMenuNode(item, `managed menus[${index}]`, null, null, state))
  validateSiblingOrder(roots, 'managed menu roots')
  return roots
}

export function parseMenuIDResult(value: unknown): MenuIDResult {
  const record = closedRecord(value, ['id'], 'menu id result')
  return { id: positiveInteger(record.id, 'menu id result id') }
}

export function parseMenuStatusResult(value: unknown): MenuStatusResult {
  const record = closedRecord(value, ['id', 'isEnabled'], 'menu status result')
  const id = positiveInteger(record.id, 'menu status result id')
  if (!isYesNo(record.isEnabled)) {
    throw new ProtocolError('menu status result isEnabled must be 0 or 1')
  }
  return { id, isEnabled: record.isEnabled }
}

function parseMenuNode(
  value: unknown,
  label: string,
  expectedParentID: number | null,
  parentType: ManagedMenuType | null,
  state: ParseState,
): ManagedMenuNode {
  const record = closedRecord(value, menuNodeKeys, label)
  const id = positiveInteger(record.id, `${label} id`)
  if (state.ids.has(id)) {
    throw new ProtocolError(`${label} duplicates menu id ${id}`)
  }
  state.ids.add(id)

  const parentId = nullablePositiveInteger(record.parentId, `${label} parentId`)
  if (parentId !== expectedParentID) {
    throw new ProtocolError(`${label} parentId does not match its enclosing node`)
  }
  const menuType = parseMenuType(record.menuType, `${label} menuType`)
  if (!isAllowedChild(parentType, menuType)) {
    throw new ProtocolError(`${label} has an invalid root or parent type`)
  }
	const name = nonEmptyTrimmedString(record.name, `${label} name`)
	if (name.length > 128) {
		throw new ProtocolError(`${label} name is too long`)
	}
	const code = nonEmptyTrimmedString(record.code, `${label} code`)
	if (code.length > 128 || !menuCodePattern.test(code)) {
		throw new ProtocolError(`${label} code has an invalid format`)
	}
  if (state.codes.has(code)) {
    throw new ProtocolError(`${label} duplicates menu code ${code}`)
  }
  state.codes.add(code)

	let i18nKey: string | null = null
	if (menuType === 'action') {
		if (record.i18nKey !== null) {
			throw new ProtocolError(`${label} action i18nKey must be null`)
		}
	} else {
		if (typeof record.i18nKey !== 'string' || !isMenuI18nKey(record.i18nKey)) {
			throw new ProtocolError(`${label} i18nKey has an invalid format`)
		}
		i18nKey = record.i18nKey
	}
	if (record.icon !== null && (typeof record.icon !== 'string' || !isMenuIcon(record.icon))) {
		throw new ProtocolError(`${label} icon has an invalid format`)
  }
  const icon = record.icon
  const sortOrder = nonNegativeInteger(record.sortOrder, `${label} sortOrder`)
	if (!isYesNo(record.isEnabled)) {
		throw new ProtocolError(`${label} isEnabled must be 0 or 1`)
	}
	if (!isYesNo(record.isHidden)) {
		throw new ProtocolError(`${label} isHidden must be 0 or 1`)
  }
	if (!isYesNo(record.isProtected)) {
		throw new ProtocolError(`${label} isProtected must be 0 or 1`)
	}
  const createdAt = timestamp(record.createdAt, `${label} createdAt`)
  const updatedAt = timestamp(record.updatedAt, `${label} updatedAt`)
  if (!Array.isArray(record.children)) {
    throw new ProtocolError(`${label} children must be an array`)
  }

	let path: string | null = null
	let componentPath: string | null = null
	if (menuType === 'directory') {
		if (record.path !== null || record.componentPath !== null) {
			throw new ProtocolError(`${label} directory path and componentPath must be null`)
		}
	} else if (menuType === 'page') {
		if (typeof record.path !== 'string' || !isMenuPath(record.path)) {
			throw new ProtocolError(`${label} path has an invalid format`)
		}
		path = record.path
		if (state.pagePaths.has(path)) {
      throw new ProtocolError(`${label} duplicates page path ${path}`)
    }
    state.pagePaths.add(path)
		if (typeof record.componentPath !== 'string' || !isComponentPath(record.componentPath)) {
			throw new ProtocolError(`${label} componentPath has an invalid format`)
		}
		componentPath = record.componentPath
	} else if (record.path !== null || record.componentPath !== null || record.icon !== null || record.isHidden !== 1) {
		throw new ProtocolError(`${label} action path, componentPath, and icon must be null and isHidden must be 1`)
  }

  const children = record.children.map((child, index) => parseMenuNode(
    child,
    `${label}.children[${index}]`,
    id,
    menuType,
    state,
  ))
  if (menuType === 'action' && children.length !== 0) {
    throw new ProtocolError(`${label} action must be a leaf`)
  }
  validateSiblingOrder(children, `${label} children`)

  return {
    id,
    parentId,
    menuType,
		name,
    code,
    i18nKey,
    path,
		componentPath,
    icon,
    sortOrder,
		isEnabled: record.isEnabled,
		isHidden: record.isHidden,
		isProtected: record.isProtected,
    createdAt,
    updatedAt,
    children,
  }
}

function parseMenuType(value: unknown, label: string): ManagedMenuType {
  if (value !== 'directory' && value !== 'page' && value !== 'action') {
    throw new ProtocolError(`${label} must be directory, page, or action`)
  }
  return value
}

function isAllowedChild(parentType: ManagedMenuType | null, childType: ManagedMenuType): boolean {
  if (parentType === null) return childType === 'directory'
  if (parentType === 'directory') return childType === 'directory' || childType === 'page'
  if (parentType === 'page') return childType === 'action'
  return false
}

function validateSiblingOrder(nodes: readonly ManagedMenuNode[], label: string): void {
  for (let index = 1; index < nodes.length; index += 1) {
    const previous = nodes[index - 1]
    const current = nodes[index]
    if (compareMenuNodes(previous, current) >= 0) {
      throw new ProtocolError(`${label} must be sorted by sortOrder, code, and id`)
    }
  }
}

function compareMenuNodes(left: ManagedMenuNode, right: ManagedMenuNode): number {
  if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
  const codeOrder = left.code.localeCompare(right.code)
  if (codeOrder !== 0) return codeOrder
  return left.id - right.id
}

function positiveInteger(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) {
    throw new ProtocolError(`${label} must be a positive integer`)
  }
  return value
}

function nullablePositiveInteger(value: unknown, label: string): number | null {
  if (value === null) return null
  return positiveInteger(value, label)
}

function nonNegativeInteger(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ProtocolError(`${label} must be a non-negative integer`)
  }
  return value
}

function nonEmptyTrimmedString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) {
    throw new ProtocolError(`${label} must be a non-empty trimmed string`)
  }
  return value
}

function timestamp(value: unknown, label: string): string {
  if (typeof value !== 'string' || !rfc3339Pattern.test(value) || !Number.isFinite(Date.parse(value))) {
    throw new ProtocolError(`${label} must be an RFC3339 timestamp`)
  }
  return value
}

function closedRecord(value: unknown, expectedKeys: readonly string[], label: string): Record<string, unknown> {
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
