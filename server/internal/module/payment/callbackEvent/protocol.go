package callbackevent

type ProcessStatus int16

const (
	ProcessPending  ProcessStatus = 1
	ProcessSuccess  ProcessStatus = 2
	ProcessIgnored  ProcessStatus = 3
	ProcessRejected ProcessStatus = 4
)

func (v ProcessStatus) Valid() bool { return v >= ProcessPending && v <= ProcessRejected }

type ErrorCode int16

const (
	ErrorConfigMismatch ErrorCode = 1
	ErrorIdentity       ErrorCode = 2
	ErrorOrderMismatch  ErrorCode = 3
	ErrorAmountMismatch ErrorCode = 4
	ErrorTradeConflict  ErrorCode = 5
	ErrorFactConflict   ErrorCode = 6
	ErrorBalance        ErrorCode = 7
	ErrorDependency     ErrorCode = 8
	ErrorUnknownStatus  ErrorCode = 9
)

func (v ErrorCode) Valid() bool { return v >= ErrorConfigMismatch && v <= ErrorUnknownStatus }
