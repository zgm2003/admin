package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestParseCronAcceptsFiveFieldsAndRejectsOtherPrecision(t *testing.T) {
	if _, err := ParseCron("*/5 * * * *"); err != nil {
		t.Fatalf("ParseCron() returned error for five-field expression: %v", err)
	}
	for _, expression := range []string{"0 */5 * * * *", "@every 30s", "*/30 * * * * *", "*/5 * * *"} {
		if _, err := ParseCron(expression); !errors.Is(err, ErrInvalidCron) {
			t.Fatalf("ParseCron(%q) error = %v, want ErrInvalidCron", expression, err)
		}
	}
}

func TestLoadTimezoneRequiresIANAName(t *testing.T) {
	if location, err := LoadTimezone("Asia/Shanghai"); err != nil || location.String() != "Asia/Shanghai" {
		t.Fatalf("LoadTimezone(Asia/Shanghai) = %v, %v", location, err)
	}
	for _, name := range []string{"", "UTC+8", "Not/A/Timezone"} {
		if _, err := LoadTimezone(name); !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("LoadTimezone(%q) error = %v, want ErrInvalidTimezone", name, err)
		}
	}
}

func TestParseCronHasNoSubMinuteNextRuns(t *testing.T) {
	schedule, err := ParseCron("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	first, second := schedule.Next(now), schedule.Next(schedule.Next(now))
	if second.Sub(first) < time.Minute {
		t.Fatalf("next run interval = %s, want at least one minute", second.Sub(first))
	}
}
