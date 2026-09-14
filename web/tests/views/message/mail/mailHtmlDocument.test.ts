import { describe, expect, it } from 'vitest'

import {
  assertSafeMailHtml,
  readMailBody,
  replaceMailVariables,
  updateMailBody,
} from '@/views/message/mail/template/components/MailHtmlEditor/mailHtmlDocument'

const documentHtml =
  '<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"></head><body style="color:red"><main style="padding:20px"><section data-mail-editor="content"><p>{{code}}</p></section></main></body></html>'

describe('mail HTML document', () => {
  it('keeps the document shell while updating only body content', () => {
    const updated = updateMailBody(documentHtml, '<strong>{{code}}</strong>')
    expect(updated).toContain('<meta charset="utf-8">')
    expect(updated).toContain('<main style="padding:20px">')
    expect(updated).toContain(
      '<section data-mail-editor="content"><strong>{{code}}</strong></section>',
    )
  })
  it('replaces example variables without changing unresolved tokens', () => {
    expect(replaceMailVariables(documentHtml, { code: '123456' })).toContain('123456')
    expect(readMailBody(documentHtml)).toBe('<p>{{code}}</p>')
  })
  it('rejects executable or non-document HTML', () => {
    expect(() =>
      assertSafeMailHtml('<!DOCTYPE html><html><body><script>alert(1)</script></body></html>'),
    ).toThrow()
    expect(() => readMailBody('<p>body only</p>')).toThrow()
    expect(() =>
      readMailBody('<!DOCTYPE html><html><head></head><body><p>no region</p></body></html>'),
    ).toThrow('可视化编辑区域')
  })
})
