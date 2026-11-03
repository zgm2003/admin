package media

import (
	"testing"
	"time"

	airun "admin/server/internal/module/ai/run"
)

func validMediaPolicy() airun.Policy {
	return airun.Policy{
		UserInflightLimit: 1, UserRatePerSecond: 1, UserRateBurst: 1, GlobalRatePerSecond: 1, GlobalRateBurst: 1,
		QueuedBackpressureThreshold: 1, ActiveBackpressureThreshold: 1, MaxQueuedSeconds: 1, TextTimeoutSeconds: 1,
		ImageTimeoutSeconds: 1, AudioTimeoutSeconds: 1, VideoTimeoutSeconds: 1, MaxModelRounds: 1, MaxToolCalls: 1,
		MaxBoundTools: 1, MaxInputAttachments: 1, MaxOutputTokens: 1, MaxTextInputBytes: 1, MaxImages: 1,
		ImageMaxBytes: 10, AudioMaxBytes: 20, VideoMaxBytes: 30, DownloadTimeoutSeconds: 4, TransferTimeoutSeconds: 9,
		SpoolMaxBytesPerWorker: 100, EventRetentionDays: 1,
	}
}

func TestMediaConstraintsUseFrozenPolicyByMIME(t *testing.T) {
	p := validMediaPolicy()
	at := time.Unix(100, 0).UTC()
	for _, tc := range []struct {
		name string
		mime string
		max  int64
	}{
		{"image", "image/png", 10},
		{"audio", "audio/mpeg", 20},
		{"video", "video/mp4", 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := mediaConstraints(File{MIMEType: &tc.mime}, p, at)
			if err != nil || c.MaxBytes != tc.max || c.DownloadTimeout != 4*time.Second || !c.Deadline.Equal(at.Add(9*time.Second)) {
				t.Fatalf("constraints=%+v err=%v", c, err)
			}
		})
	}
}

func TestMediaConstraintsRejectMissingFrozenLimits(t *testing.T) {
	p := validMediaPolicy()
	p.AudioMaxBytes = 0
	mime := "audio/mpeg"
	if _, err := mediaConstraints(File{MIMEType: &mime}, p, time.Now()); err == nil {
		t.Fatal("missing audio limit accepted")
	}
}
