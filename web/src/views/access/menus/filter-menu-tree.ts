import type { ManagedMenuNode } from '../../../api/menu.contract'

function cloneNode(node: ManagedMenuNode): ManagedMenuNode {
  return { ...node, children: node.children.map(cloneNode) }
}

export function filterManagedMenuTree(
  nodes: readonly ManagedMenuNode[],
  rawKeyword: string,
): ManagedMenuNode[] {
  const keyword = rawKeyword.trim().toLocaleLowerCase()
  if (keyword === '') return nodes.map(cloneNode)
  return nodes.flatMap((node) => {
    const ownMatch = [node.name, node.code, node.path ?? '']
      .some((value) => value.toLocaleLowerCase().includes(keyword))
    if (ownMatch) return [cloneNode(node)]
    const children = filterManagedMenuTree(node.children, keyword)
    return children.length === 0 ? [] : [{ ...node, children }]
  })
}
