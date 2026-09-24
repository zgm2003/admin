package loginlog

import (
	"strings"

	"admin/server/internal/shared/yesno"
)

type Event struct {
	UserID     *int64
	PlatformID int64
	Account    string
	EventType  EventType
	LoginType  *LoginType
	IsSuccess  yesno.Value
	ReasonCode string
	ClientIP   string
	UserAgent  string
}

func ValidateEvent(event Event) error {
	if event.PlatformID < 1 || strings.TrimSpace(event.Account) == "" || len(event.Account) > 254 {
		return errInvalidEvent
	}
	if !event.EventType.IsValid() {
		return errInvalidEvent
	}
	if event.EventType == EventRegister || event.EventType == EventLogin {
		if event.LoginType == nil || !event.LoginType.IsValid() {
			return errInvalidEvent
		}
		if event.EventType == EventRegister && *event.LoginType == LoginPassword {
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

type EventType int16

const (
	EventRegister EventType = 1
	EventLogin    EventType = 2
	EventLogout   EventType = 3
)

func (value EventType) IsValid() bool {
	return value == EventRegister || value == EventLogin || value == EventLogout
}

type LoginType int16

const (
	LoginPassword LoginType = 1
	LoginEmail    LoginType = 2
	LoginPhone    LoginType = 3
)

func (value LoginType) IsValid() bool {
	return value == LoginPassword || value == LoginEmail || value == LoginPhone
}
