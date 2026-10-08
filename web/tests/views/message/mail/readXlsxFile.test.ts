import { afterEach, describe, expect, it, vi } from 'vitest'
import { readXlsxFile } from '@/views/message/mail/recipientRule/components/MailRuleImportDialog/readXlsxFile'

describe('local XLSX binary reading', () => {
  afterEach(() => vi.restoreAllMocks())

  it('encodes binary bytes without decoding them as text', async () => {
    const file = new File([new Uint8Array([80, 75, 3, 4, 255, 0])], 'rules.xlsx')
    await expect(readXlsxFile(file, new AbortController().signal)).resolves.toBe('UEsDBP8A')
  })

  it('reads a 2 MiB file in bounded encoding chunks', async () => {
    const bytes = new Uint8Array(2 * 1024 * 1024)
    bytes.set([80, 75, 3, 4])
    const encoded = await readXlsxFile(
      new File([bytes], 'rules.xlsx'),
      new AbortController().signal,
    )
    expect(encoded.length).toBe(2796204)
    expect(atob(encoded).length).toBe(2097152)
    expect(atob(encoded).slice(0, 4)).toBe('PK\x03\x04')
  })

  it('aborts the native read when its dialog request is cancelled', async () => {
    const controller = new AbortController()
    const abort = vi.spyOn(FileReader.prototype, 'abort')
    const result = readXlsxFile(new File([new Uint8Array(1024)], 'rules.xlsx'), controller.signal)
    controller.abort()
    await expect(result).rejects.toMatchObject({ name: 'AbortError' })
    expect(abort).toHaveBeenCalledOnce()
  })

  it('does not start a read for an already cancelled selection', async () => {
    const controller = new AbortController()
    controller.abort()
    const read = vi.spyOn(FileReader.prototype, 'readAsArrayBuffer')
    await expect(
      readXlsxFile(new File(['data'], 'rules.xlsx'), controller.signal),
    ).rejects.toMatchObject({ name: 'AbortError' })
    expect(read).not.toHaveBeenCalled()
  })

  it('rejects native read errors instead of resolving empty content', async () => {
    vi.spyOn(FileReader.prototype, 'readAsArrayBuffer').mockImplementation(function (
      this: FileReader,
    ) {
      this.dispatchEvent(new ProgressEvent('error'))
    })
    await expect(
      readXlsxFile(new File(['data'], 'rules.xlsx'), new AbortController().signal),
    ).rejects.toThrow('could not be read')
  })

  it('rejects malformed reader output and synchronous failures', async () => {
    const read = vi.spyOn(FileReader.prototype, 'readAsArrayBuffer').mockImplementation(function (
      this: FileReader,
    ) {
      this.dispatchEvent(new ProgressEvent('load'))
    })
    await expect(
      readXlsxFile(new File(['data'], 'rules.xlsx'), new AbortController().signal),
    ).rejects.toThrow('could not be read')
    read.mockImplementation(() => {
      throw new Error('read failed')
    })
    await expect(
      readXlsxFile(new File(['data'], 'rules.xlsx'), new AbortController().signal),
    ).rejects.toThrow('could not be read')
  })
})
