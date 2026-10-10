import { ElMessageBox } from 'element-plus'

import { createUploadRule, updateUploadRule } from '@/api/storage/uploadRule'
import { YesNo } from '@/enums/yesNo'
import type { RuleForm } from './components/types'

type Translate = (key: string) => string

export async function saveExistingRule(
  id: number,
  form: RuleForm,
  canUpdateStatus: boolean,
  mutable: Omit<Parameters<typeof updateUploadRule>[1], 'isEnabled'>,
  t: Translate,
): Promise<void> {
  if (canUpdateStatus && form.isEnabled === YesNo.Yes) {
    await ElMessageBox.confirm(t('storage.confirmEnabledRuleReplacement'), t('storage.status'), {
      type: 'warning',
    })
  }
  await updateUploadRule(id, {
    ...mutable,
    ...(canUpdateStatus ? { isEnabled: form.isEnabled } : {}),
  })
}

export async function saveNewRule(
  form: RuleForm,
  mutable: Parameters<typeof createUploadRule>[0],
  t: Translate,
): Promise<void> {
  if (form.isEnabled === YesNo.Yes) {
    await ElMessageBox.confirm(t('storage.confirmEnabledRuleReplacement'), t('storage.status'), {
      type: 'warning',
    })
  }
  await createUploadRule(mutable)
}
