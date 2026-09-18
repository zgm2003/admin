import { ElMessageBox } from 'element-plus'

import {
  createUploadRule,
  updateUploadRule,
  updateUploadRuleStatus,
} from '@/api/storage/uploadRule'
import { YesNo } from '@/enums/yesNo'
import type { UploadRule } from '@/api/storage/uploadRule'
import type { RuleForm } from './components/types'

type Translate = (key: string) => string

export async function saveExistingRule(
  id: number,
  form: RuleForm,
  current: UploadRule | undefined,
  rules: readonly UploadRule[],
  canUpdateStatus: boolean,
  mutable: Parameters<typeof updateUploadRule>[1],
  t: Translate,
): Promise<void> {
  const statusChanged =
    canUpdateStatus && current !== undefined && current.isEnabled !== form.isEnabled
  if (
    statusChanged &&
    form.isEnabled === YesNo.Yes &&
    rules.some(
      (rule) =>
        rule.id !== id && rule.platformId === form.platformId && rule.isEnabled === YesNo.Yes,
    )
  ) {
    await ElMessageBox.confirm(t('storage.confirmEnabledRuleReplacement'), t('storage.status'), {
      type: 'warning',
    })
  }
  await updateUploadRule(id, mutable)
  if (statusChanged) await updateUploadRuleStatus(id, form.isEnabled)
}

export async function saveNewRule(
  form: RuleForm,
  rules: readonly UploadRule[],
  mutable: Parameters<typeof createUploadRule>[0],
  t: Translate,
): Promise<void> {
  if (
    form.isEnabled === YesNo.Yes &&
    rules.some((rule) => rule.platformId === form.platformId && rule.isEnabled === YesNo.Yes)
  ) {
    await ElMessageBox.confirm(t('storage.confirmEnabledRuleReplacement'), t('storage.status'), {
      type: 'warning',
    })
  }
  await createUploadRule(mutable)
}
