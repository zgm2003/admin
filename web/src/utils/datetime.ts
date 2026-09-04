/**
 * Format an API timestamp string for display in tables and detail views.
 * Invalid or unparsable values render as "-" instead of throwing.
 */
export function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(
    date,
  )
}
