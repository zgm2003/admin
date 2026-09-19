package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"testing"
	"time"
)

func TestDraftInputValidatesAudienceTargetsAndSchedule(t *testing.T) {
	now := time.Now().UTC()
	base := DraftInput{PlatformID: 1, Title: "title", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: []int64{2, 1, 2}, ScheduledAt: &now}
	normalized, err := NormalizeDraft(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.TargetIDs) != 2 || normalized.TargetIDs[0] != 1 || normalized.TargetIDs[1] != 2 {
		t.Fatalf("targets=%v", normalized.TargetIDs)
	}
	invalid := []DraftInput{func() DraftInput { v := base; v.AudienceType = AudiencePlatform; return v }(), func() DraftInput { v := base; v.AudienceType = AudienceRole; v.TargetIDs = nil; return v }(), func() DraftInput { v := base; v.PlatformID = 0; return v }()}
	for _, v := range invalid {
		if _, err := NormalizeDraft(v); err == nil {
			t.Fatalf("NormalizeDraft(%+v) error=nil", v)
		}
	}
}
