import type { ManagedMenuNode, ManagedMenuType } from '@/api/permission/menu'

export function menuRowKey(id: number): string {
  return String(id)
}

export function flattenWithChildren(nodes: readonly ManagedMenuNode[]): ManagedMenuNode[] {
  const result: ManagedMenuNode[] = []
  const stack = [...nodes].reverse()
  while (stack.length > 0) {
    const node = stack.pop()
    if (node === undefined) continue
    result.push(node)
    stack.push(...[...node.children].reverse())
  }
  return result
}

export function collectSubtreeIDs(node: ManagedMenuNode): Set<number> {
  return new Set(flattenWithChildren([node]).map((item) => item.id))
}

export function menuParentOptions(
  nodes: readonly ManagedMenuNode[],
  menuType: ManagedMenuType,
  editingID: number | null,
): ManagedMenuNode[] {
  const editingNode =
    editingID === null
      ? null
      : (flattenWithChildren(nodes).find((node) => node.id === editingID) ?? null)
  const excluded = editingNode === null ? new Set<number>() : collectSubtreeIDs(editingNode)

  return flattenWithChildren(nodes).filter((node) => {
    if (excluded.has(node.id)) return false
    return menuType === 'action' ? node.menuType === 'page' : node.menuType === 'directory'
  })
}
