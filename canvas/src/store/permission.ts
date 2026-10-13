import { getPermission, type PermissionMenuNode, type PermissionSnapshot } from '@/api/permission/permission'
import type { RequestClient } from '@/utils/request'
import { ProtocolError } from '@/types/http'

export interface PermissionState extends PermissionSnapshot { status: 'idle' | 'loading' | 'ready' | 'error'; error: string }
const empty = (): PermissionState => ({ status: 'idle', error: '', roleCodes: [], permissionCodes: [], menuTree: [] })
export class PermissionStore {
  state = empty()
  private listeners = new Set<() => void>()
  private generation = 0
  private pending: Promise<void> | null = null
  constructor(private client: RequestClient) {}
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener) } }
  getSnapshot = () => this.state
  hasPermission = (code: string) => this.state.status === 'ready' && this.state.permissionCodes.includes(code)
  hasPage = (componentPath: string, code: string) => this.hasPermission(code) && this.pages().some(node => node.componentPath === componentPath && node.code === code)
  pages(): PermissionMenuNode[] {
    const walk = (nodes: PermissionMenuNode[]): PermissionMenuNode[] => nodes.flatMap(node => node.menuType === 'page' ? [node] : walk(node.children))
    return this.state.status === 'ready' ? walk(this.state.menuTree) : []
  }
  reset() { this.generation++; this.pending = null; this.set(empty()) }
  load(force = false): Promise<void> {
    if (this.pending) return this.pending
    if (!force && this.state.status === 'ready') return Promise.resolve()
    this.set({ ...empty(), status: 'loading' })
    const generation = this.generation
    const pending = getPermission(this.client).then(value => {
      const snapshot = parseSnapshot(value)
      if (generation === this.generation) this.set({ ...snapshot, status: 'ready', error: '' })
    }).catch((error: unknown) => {
      if (generation === this.generation) this.set({ ...empty(), status: 'error', error: error instanceof Error ? error.message : '权限加载失败' })
      throw error
    }).finally(() => { if (this.pending === pending) this.pending = null })
    this.pending = pending
    return pending
  }
  private set(state: PermissionState) { this.state = state; this.listeners.forEach(listener => listener()) }
}

function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function strings(value: unknown): value is string[] { return Array.isArray(value) && value.every(item => typeof item === 'string' && item.trim() !== '') }
function parseSnapshot(value: unknown): PermissionSnapshot {
  const invalid = () => new ProtocolError('权限响应格式错误，请重新加载')
  if (!record(value) || !strings(value.roleCodes) || !strings(value.permissionCodes) || !Array.isArray(value.menuTree)) throw invalid()
  const codes = new Set(value.permissionCodes)
  const seen = new Set<string>()
  const parseNode = (value: unknown, depth: number): PermissionMenuNode => {
    if (depth > 64 || !record(value) || typeof value.code !== 'string' || !value.code || seen.has(value.code) ||
      (value.menuType !== 'directory' && value.menuType !== 'page') || typeof value.i18nKey !== 'string' || !value.i18nKey ||
      (value.icon !== null && typeof value.icon !== 'string') || (value.isHidden !== 0 && value.isHidden !== 1) || !Array.isArray(value.children)) throw invalid()
    seen.add(value.code)
    const children = value.children.map(child => parseNode(child, depth + 1))
    if (value.menuType === 'page') {
      if (typeof value.path !== 'string' || typeof value.componentPath !== 'string' || value.path !== '/' + value.componentPath || !codes.has(value.code) || children.length > 0) throw invalid()
    } else if (value.path !== null || value.componentPath !== null) throw invalid()
    return { code: value.code, menuType: value.menuType, path: value.path as string | null, componentPath: value.componentPath as string | null, i18nKey: value.i18nKey, icon: value.icon, isHidden: value.isHidden, children }
  }
  return { roleCodes: [...value.roleCodes], permissionCodes: [...value.permissionCodes], menuTree: value.menuTree.map(node => parseNode(node, 0)) }
}
