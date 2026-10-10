import type { NotificationTaskAdminOptions } from '@/api/message/notificationTaskOptions'

export const taskCatalog: NotificationTaskAdminOptions = {
  defaults: { variant: 'info', priority: 'normal', linkType: 'none', audienceType: 'platform' },
  constraints: { titleMaxLength: 128 },
  statuses: [
    { value: 1, label: '草稿' },
    { value: 2, label: '待调度' },
    { value: 3, label: '已入队' },
    { value: 4, label: '处理中' },
    { value: 5, label: '已完成' },
    { value: 6, label: '失败' },
    { value: 7, label: '已取消' },
    { value: 99, label: '未来任务状态' },
  ],
  audiences: [
    { value: 'platform', label: '整个平台' },
    { value: 'user', label: '指定用户' },
    { value: 'role', label: '指定角色' },
  ],
  variants: [
    { value: 'info', label: '信息' },
    { value: 'warning', label: '警告' },
  ],
  priorities: [
    { value: 'normal', label: '普通' },
    { value: 'urgent', label: '紧急' },
  ],
  linkTypes: [
    { value: 'none', label: '无链接' },
    { value: 'internal', label: '站内链接' },
    { value: 'external', label: '外部链接' },
  ],
}

export const draftActions = { edit: true, delete: true, submit: true, cancel: false, copy: false }
export const completedActions = {
  edit: false,
  delete: false,
  submit: false,
  cancel: false,
  copy: true,
}
