package media

import (
	"testing"
	"time"

	"admin/server/internal/module/ai"
)

func TestMediaConstraintsUseFixedMIMELimits(t *testing.T) {
	at := time.Unix(100, 0).UTC()
	for _, tc := range []struct {
		name string
		mime string
		max  int64
	}{
		{"image", "image/png", ai.ImageMaxBytes},
		{"audio", "audio/mpeg", ai.AudioMaxBytes},
		{"video", "video/mp4", ai.VideoMaxBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := mediaConstraints(File{MIMEType: &tc.mime}, at)
			if err != nil || c.MaxBytes != tc.max || c.DownloadTimeout != time.Duration(ai.DownloadTimeoutSeconds)*time.Second || !c.Deadline.Equal(at.Add(time.Duration(ai.TransferTimeoutSeconds)*time.Second)) {
				t.Fatalf("constraints=%+v err=%v", c, err)
			}
		})
	}
}
