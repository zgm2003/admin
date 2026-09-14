const forbidden = /<(script|iframe|object|embed|form)(\s|>)/i
const eventAttribute = /\son[a-z]+\s*=/i
const javascriptURL = /javascript\s*:/i

export function assertSafeMailHtml(html: string): void {
  if (
    !/^\s*<!doctype html>/i.test(html) ||
    forbidden.test(html) ||
    eventAttribute.test(html) ||
    javascriptURL.test(html)
  ) {
    throw new Error('邮件 HTML 包含不允许的结构')
  }
}

export function replaceMailVariables(html: string, values: Record<string, string>): string {
  return html.replace(
    /\{\{([a-z][a-z0-9_]*)\}\}/g,
    (_match, key: string) => values[key] ?? `{{${key}}}`,
  )
}

export function readMailBody(html: string): string {
  assertSafeMailHtml(html)
  const document = new DOMParser().parseFromString(html, 'text/html')
  if (!document.doctype || !document.head || !document.body) throw new Error('邮件 HTML 结构不完整')
  return document.body.innerHTML
}

export function updateMailBody(html: string, body: string): string {
  assertSafeMailHtml(html)
  const document = new DOMParser().parseFromString(html, 'text/html')
  if (!document.doctype || !document.head || !document.body) throw new Error('邮件 HTML 结构不完整')
  document.body.innerHTML = body
  return `<!DOCTYPE html>\n${document.documentElement.outerHTML}`
}
