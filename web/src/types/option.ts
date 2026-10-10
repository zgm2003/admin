export interface BackendOption<T extends string | number | boolean> {
  value: T
  label: string
}

export interface StatusOption extends BackendOption<number> {
  tone: 'info' | 'success' | 'warning' | 'danger'
}
