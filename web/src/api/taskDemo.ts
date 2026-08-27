import { request } from '../utils/request'

export interface CreateExampleTaskInput {
  message: string
}

export interface CreatedTask {
  taskId: string
}

export async function createExampleTask(input: CreateExampleTaskInput): Promise<CreatedTask> {
  return request<CreatedTask>({
    method: 'POST',
    url: '/api/admin/v1/example-tasks',
    data: input,
  })
}
