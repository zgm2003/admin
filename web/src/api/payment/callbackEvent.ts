import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface CallbackPayload {
  notifyId: string | null
  orderNo: string
  tradeNo: string | null
  tradeStatus: string
  appId: string
  sellerId: string | null
  totalAmount: string
  paidAt: string | null
}

export interface CallbackEvent {
  id: number
  configId: number
  configVersion: number
  orderId: number | null
  payload: CallbackPayload
  processStatus: number
  errorCode: number | null
  createdAt: string
  updatedAt: string
}

export function getOrderCallbackEvents(
  orderId: number,
  query: PageRequest,
): Promise<PageResult<CallbackEvent>> {
  return request.get<PageResult<CallbackEvent>>(`/api/admin/v1/payment/order/${orderId}/callback`, {
    params: query,
  })
}

export function getOrderCallbackEvent(orderId: number, eventId: number): Promise<CallbackEvent> {
  return request.get<CallbackEvent>(
    `/api/admin/v1/payment/order/${orderId}/callback/${eventId}`,
  )
}
