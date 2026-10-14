export class ProtocolError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ProtocolError'
  }
}

export class HttpError extends Error {
  readonly httpStatus: number

  constructor(httpStatus: number, message: string) {
    super(message)
    this.name = 'HttpError'
    this.httpStatus = httpStatus
  }
}

export class ApiError extends Error {
  readonly code: number
  readonly httpStatus: number

  constructor(code: number, message: string, httpStatus: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.httpStatus = httpStatus
  }
}
