import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createDictionary,
  deleteDictionary,
  getDictionaries,
  getDictionaryOptions,
  updateDictionary,
  updateDictionaryItem,
  updateDictionaryStatus,
} from '@/api/system/dictionary'
import { YesNo } from '@/enums/yesNo'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('system dictionary API', () => {
  beforeEach(() => requestMock.mockReset())

  it('rejects unknown fields in the page response', async () => {
    requestMock.mockResolvedValueOnce({
      list: [],
      total: 0,
      page: 1,
      pageSize: 20,
      ignored: true,
    })

    await expect(getDictionaries({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(ProtocolError)
  })

  it('requires the options response to contain exactly the requested codes', async () => {
    requestMock.mockResolvedValueOnce({ 'user.gender': [] })
    await expect(getDictionaryOptions(['user.gender', 'user.status'])).rejects.toBeInstanceOf(
      ProtocolError,
    )

    requestMock.mockResolvedValueOnce({ 'user.gender': [], unexpected: [] })
    await expect(getDictionaryOptions(['user.gender'])).rejects.toBeInstanceOf(ProtocolError)
  })

  it('parses exact create and mutation responses', async () => {
    requestMock
      .mockResolvedValueOnce({ id: 7, ignored: true })
      .mockResolvedValueOnce({ id: 7, isEnabled: YesNo.No, ignored: true })
      .mockResolvedValueOnce({})

    await expect(
      createDictionary({
        code: 'user.gender',
        nameZh: '性别',
        nameEn: 'Gender',
        description: '',
      }),
    ).rejects.toBeInstanceOf(ProtocolError)
    await expect(updateDictionaryStatus(7, YesNo.No)).rejects.toBeInstanceOf(ProtocolError)
    await expect(deleteDictionary(7)).resolves.toBeUndefined()
  })

  it('allowlists immutable code and value out of update bodies', async () => {
    requestMock.mockResolvedValue({})
    const dictionary = {
      code: 'immutable.code',
      nameZh: '名称',
      nameEn: 'Name',
      description: '',
    }
    const item = { value: 'immutable', labelZh: '标签', labelEn: 'Label', sort: 1 }

    await updateDictionary(9, dictionary)
    await updateDictionaryItem(9, 10, item)

    expect(requestMock.mock.calls[0]?.[0]).toEqual({
      method: 'PUT',
      url: '/api/admin/v1/system/dictionary/9',
      data: { nameZh: '名称', nameEn: 'Name', description: '' },
    })
    expect(requestMock.mock.calls[1]?.[0]).toEqual({
      method: 'PUT',
      url: '/api/admin/v1/system/dictionary/9/item/10',
      data: { labelZh: '标签', labelEn: 'Label', sort: 1 },
    })
  })
})
