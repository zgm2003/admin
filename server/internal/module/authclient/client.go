package authclient

import (
	"fmt"
	"regexp"
)

const (
	PlatformHeader = "X-Auth-Platform"
	DeviceIDHeader = "X-Device-ID"
)

var (
	platformPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,48}$`)
	deviceIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Client struct {
	Platform  string
	DeviceID  string
	ClientIP  string
	UserAgent string
}

func ValidatePlatform(value string) error {
	if !platformPattern.MatchString(value) {
		return fmt.Errorf("authentication platform is invalid")
	}
	return nil
}

func ValidateDeviceID(value string) error {
	if !deviceIDPattern.MatchString(value) {
		return fmt.Errorf("device ID is invalid")
	}
	return nil
}
