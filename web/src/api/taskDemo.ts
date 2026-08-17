import { ProtocolError, request } from '../utils/request'

export interface CreateExampleTaskInput {
  message: string
}

export interface CreatedTask {
  taskId: string
}

export async function createExampleTask(input: CreateExampleTaskInput): Promise<CreatedTask> {
  const data = await request<unknown>({
    method: 'POST',
    url: '/api/v1/example-tasks',
    data: input,
  })
  if (!isCreatedTask(data)) {
    throw new ProtocolError('POST /api/v1/example-tasks returned invalid data')
  }
  return data
}

function isCreatedTask(value: unknown): value is CreatedTask {
  return isRecord(value) && typeof value.taskId === 'string' && value.taskId.trim() !== ''
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
