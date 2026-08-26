import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { createExampleTask } from '@src/api/taskDemo'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const mockedRequest = vi.mocked(request)

describe('task demo API', () => {
  beforeEach(() => mockedRequest.mockReset())

  it('returns the backend task DTO directly', async () => {
    const data = { taskId: 'task-1' }
    mockedRequest.mockResolvedValue(data)
    await expect(createExampleTask({ message: 'foundation-check' })).resolves.toBe(data)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/example-tasks',
      data: { message: 'foundation-check' },
    })
  })
})
