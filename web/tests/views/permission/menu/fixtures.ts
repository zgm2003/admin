import type { MenuOptions } from '@/api/permission/menuOptions'
export const menuOptionsFixture: MenuOptions = {
  menuTypes: [
    { value: 'directory', label: '目录' },
    { value: 'page', label: '页面' },
    { value: 'action', label: '操作' },
  ],
  constraints: {
    codePattern: '^[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*)*$',
    i18nKeyPattern: '^[a-z][a-z0-9]*(?:\\.[a-z][a-zA-Z0-9]*)+$',
    pathPattern: '^/[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*)*$',
    componentPathPattern:
      '^[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-zA-Z0-9]*(?:-[a-z0-9]+)*)*$',
    nameMaxLength: 128,
    codeMaxLength: 128,
    i18nKeyMaxLength: 128,
    pathMaxLength: 255,
    reservedPagePaths: ['/dashboard', '/login', '/register'],
  },
}

export function menuRowDisplay(type: 'directory' | 'page' | 'action') {
  const allowedChildTypes: Array<'directory' | 'page' | 'action'> =
    type === 'directory' ? ['directory', 'page'] : type === 'page' ? ['action'] : []
  return {
    presentation: {
      typeLabel: { directory: '目录', page: '页面', action: '按钮权限' }[type],
      typeTone: 'success' as const,
      visibilityLabel: '显示',
      visibilityTone: 'success' as const,
      statusLabel: '已启用',
      statusTone: 'success' as const,
      statusActionLabel: '禁用',
      protectionReason: '',
      statusReason: '',
    },
    actions: {
      update: true,
      status: true,
      delete: true,
      addChild: allowedChildTypes.length > 0,
      allowedChildTypes,
    },
  }
}
