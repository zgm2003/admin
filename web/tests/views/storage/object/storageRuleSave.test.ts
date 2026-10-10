import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ElMessageBox } from 'element-plus'
import {
  createUploadRule,
  updateUploadRule,
  updateUploadRuleStatus,
} from '@/api/storage/uploadRule'
import { saveExistingRule, saveNewRule } from '@/views/storage/object/storageRuleSave'
import { YesNo } from '@/enums/yesNo'
import type { RuleForm } from '@/views/storage/object/components/types'
vi.mock('@/api/storage/uploadRule', () => ({
  createUploadRule: vi.fn(),
  updateUploadRule: vi.fn(),
  updateUploadRuleStatus: vi.fn(),
}))
vi.mock('element-plus', () => ({ ElMessageBox: { confirm: vi.fn() } }))
const mutable = {
  codes: ['avatar'],
  name: 'Avatar',
  maxFileSizeBytes: 1024,
  allowedExtensions: ['png'],
  allowedMimeTypes: ['image/png'],
  remark: '',
}
const form: RuleForm = {
  ...mutable,
  platformId: 1,
  cosConfigId: 2,
  accessMode: 'private',
  isEnabled: YesNo.Yes,
}
const t = (key: string) => key
describe('atomic upload rule saving', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(ElMessageBox.confirm).mockResolvedValue(
      Object.assign('confirm' as const, { value: '', action: 'confirm' as const }),
    )
    vi.mocked(updateUploadRule).mockResolvedValue({})
    vi.mocked(createUploadRule).mockResolvedValue({ id: 7 })
  })
  it('sends fields and requested status in one write, without querying stale list facts', async () => {
    await saveExistingRule(7, form, true, mutable, t)
    expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
    expect(updateUploadRule).toHaveBeenCalledExactlyOnceWith(7, {
      ...mutable,
      isEnabled: YesNo.Yes,
    })
    expect(updateUploadRuleStatus).not.toHaveBeenCalled()
  })
  it('does not send status for a user without status permission', async () => {
    await saveExistingRule(7, form, false, mutable, t)
    expect(updateUploadRule).toHaveBeenCalledExactlyOnceWith(7, mutable)
    expect(ElMessageBox.confirm).not.toHaveBeenCalled()
    expect(updateUploadRuleStatus).not.toHaveBeenCalled()
  })
  it('cancellation prevents the only write', async () => {
    vi.mocked(ElMessageBox.confirm).mockRejectedValue('cancel')
    await expect(saveExistingRule(7, form, true, mutable, t)).rejects.toBe('cancel')
    expect(updateUploadRule).not.toHaveBeenCalled()
  })
  it('propagates the single-write failure without a second status request', async () => {
    const error = new Error('write failed')
    vi.mocked(updateUploadRule).mockRejectedValue(error)
    await expect(saveExistingRule(7, form, true, mutable, t)).rejects.toBe(error)
    expect(updateUploadRule).toHaveBeenCalledOnce()
    expect(updateUploadRuleStatus).not.toHaveBeenCalled()
  })
  it('sends disabled status in the same write without replacement confirmation', async () => {
    await saveExistingRule(7, { ...form, isEnabled: YesNo.No }, true, mutable, t)
    expect(updateUploadRule).toHaveBeenCalledExactlyOnceWith(7, { ...mutable, isEnabled: YesNo.No })
    expect(ElMessageBox.confirm).not.toHaveBeenCalled()
  })
  it('confirms enabled creation even with no stale list row', async () => {
    await saveNewRule(form, { ...form }, t)
    expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
    expect(createUploadRule).toHaveBeenCalledOnce()
  })
})
