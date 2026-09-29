import { computed, ref } from 'vue'

import { getEmailChangeLogs, type EmailChangeLogItem } from '@/api/user/email'
import { getPhoneChangeLogs, type PhoneChangeLogItem } from '@/api/user/phone'
import type { TablePaginationState } from '@/components/AppTable'
import type { UserListItem } from '@/api/user/account'

type ErrorMessage = (error: unknown, fallback: string) => string

export function useIdentityChangeLogs(errorMessage: ErrorMessage) {
  const visible = ref(false)
  const target = ref<UserListItem | null>(null)
  const emailRows = ref<EmailChangeLogItem[]>([])
  const emailTotal = ref(0)
  const emailQuery = ref({ page: 1, pageSize: 20 })
  const emailLoading = ref(false)
  const emailError = ref('')
  const phoneRows = ref<PhoneChangeLogItem[]>([])
  const phoneTotal = ref(0)
  const phoneQuery = ref({ page: 1, pageSize: 20 })
  const phoneLoading = ref(false)
  const phoneError = ref('')
  let emailRequestSequence = 0
  let phoneRequestSequence = 0

  const emailPagination = computed<TablePaginationState>(() => ({
    currentPage: emailQuery.value.page,
    pageSize: emailQuery.value.pageSize,
    total: emailTotal.value,
  }))
  const phonePagination = computed<TablePaginationState>(() => ({
    currentPage: phoneQuery.value.page,
    pageSize: phoneQuery.value.pageSize,
    total: phoneTotal.value,
  }))

  async function loadEmail(): Promise<void> {
    if (target.value === null) return
    const requestSequence = ++emailRequestSequence
    const userID = target.value.id
    const query = { ...emailQuery.value }
    emailLoading.value = true
    emailError.value = ''
    try {
      const result = await getEmailChangeLogs(userID, query)
      if (requestSequence !== emailRequestSequence) return
      emailRows.value = result.list
      emailTotal.value = result.total
    } catch (error: unknown) {
      if (requestSequence !== emailRequestSequence) return
      emailError.value = errorMessage(error, 'user.emailChangeLoadFailed')
    } finally {
      if (requestSequence === emailRequestSequence) emailLoading.value = false
    }
  }

  async function loadPhone(): Promise<void> {
    if (target.value === null) return
    const requestSequence = ++phoneRequestSequence
    const userID = target.value.id
    const query = { ...phoneQuery.value }
    phoneLoading.value = true
    phoneError.value = ''
    try {
      const result = await getPhoneChangeLogs(userID, query)
      if (requestSequence !== phoneRequestSequence) return
      phoneRows.value = result.list
      phoneTotal.value = result.total
    } catch (error: unknown) {
      if (requestSequence !== phoneRequestSequence) return
      phoneError.value = errorMessage(error, 'user.phoneChangeLoadFailed')
    } finally {
      if (requestSequence === phoneRequestSequence) phoneLoading.value = false
    }
  }

  async function open(row: UserListItem): Promise<void> {
    target.value = row
    emailRows.value = []
    emailTotal.value = 0
    emailQuery.value = { page: 1, pageSize: 20 }
    phoneRows.value = []
    phoneTotal.value = 0
    phoneQuery.value = { page: 1, pageSize: 20 }
    emailError.value = ''
    phoneError.value = ''
    visible.value = true
    await Promise.all([loadEmail(), loadPhone()])
  }

  function updateEmailPagination(next: TablePaginationState): void {
    emailQuery.value = { page: next.currentPage, pageSize: next.pageSize }
    void loadEmail()
  }
  function updatePhonePagination(next: TablePaginationState): void {
    phoneQuery.value = { page: next.currentPage, pageSize: next.pageSize }
    void loadPhone()
  }

  return {
    visible,
    target,
    emailRows,
    emailLoading,
    emailError,
    emailPagination,
    phoneRows,
    phoneLoading,
    phoneError,
    phonePagination,
    open,
    refreshEmail: loadEmail,
    refreshPhone: loadPhone,
    updateEmailPagination,
    updatePhonePagination,
  }
}
