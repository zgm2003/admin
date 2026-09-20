package scheduler

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var fiveFieldCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func ParseCron(expression string) (cron.Schedule, error) {
	expression = strings.TrimSpace(expression)
	if len(strings.Fields(expression)) != 5 {
		return nil, ErrInvalidCron
	}
	schedule, err := fiveFieldCronParser.Parse(expression)
	if err != nil {
		return nil, ErrInvalidCron
	}
	now := time.Now().UTC()
	first := schedule.Next(now)
	second := schedule.Next(first)
	if first.IsZero() || second.IsZero() || second.Sub(first) < time.Minute {
		return nil, ErrInvalidCron
	}
	return schedule, nil
}

func LoadTimezone(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidTimezone
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, ErrInvalidTimezone
	}
	return location, nil
}
