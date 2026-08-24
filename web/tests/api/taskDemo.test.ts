import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { createExampleTask } from '@src/api/taskDemo'

vi.mock('@src/utils/request', () => ({
  ProtocolError: class ProtocolError extends Error {},
  request: vi.fn(),
}))

const mockedRequest = vi.mocked(request)

describe('task demo API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it.each([{}, { taskId: '' }, { taskId: '   ' }])(
    'rejects malformed created task data: %j',
    async (data) => {
      mockedRequest.mockResolvedValue(data)

      await expect(createExampleTask({ message: 'foundation-check' })).rejects.toThrow(
        'POST /api/v1/example-tasks returned invalid data',
      )
    },
  )
})
