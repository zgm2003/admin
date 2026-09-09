package loginlog

import "admin/server/internal/shared/yesno"

type Event struct {
	UserID       *int64
	SessionID    *int64
	PlatformID   int64
	LoginAccount string
	EventType    string
	LoginType    *string
	IsSuccess    yesno.Value
	ReasonCode   string
	ClientIP     string
	UserAgent    string
}

func ValidateEvent(event Event) error {
	if event.PlatformID < 1 || len(event.LoginAccount) > 254 || (event.EventType == EventLogin && event.LoginAccount == "") {
		return errInvalidEvent
	}
	if event.EventType != EventLogin && event.EventType != EventLogout {
		return errInvalidEvent
	}
	if event.EventType == EventLogin {
		if event.LoginType == nil || *event.LoginType == "" || len(*event.LoginType) > 32 {
			return errInvalidEvent
		}
	} else if event.LoginType != nil {
		return errInvalidEvent
	}
	if !yesno.IsValid(event.IsSuccess) || event.ReasonCode == "" || len(event.ReasonCode) > 64 || len(event.ClientIP) > 64 || len(event.UserAgent) > 512 {
		return errInvalidEvent
	}
	return nil
}
