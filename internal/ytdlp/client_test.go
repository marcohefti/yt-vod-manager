package ytdlp

import "testing"

func TestSelectFormat(t *testing.T) {
	tests := []struct {
		name       string
		quality    string
		fragmented bool
		want       string
	}{
		{name: "best auto", quality: "best", fragmented: false, want: "bv*+ba/b"},
		{name: "best fragmented", quality: "best", fragmented: true, want: "bv*[protocol*=m3u8]+ba[protocol*=m3u8]/b[protocol*=m3u8]"},
		{name: "1080 auto", quality: "1080p", fragmented: false, want: "bv*[height<=1080]+ba/b[height<=1080]"},
		{name: "720 fragmented", quality: "720p", fragmented: true, want: "bv*[protocol*=m3u8][height<=720]+ba[protocol*=m3u8]/b[protocol*=m3u8][height<=720]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectFormat(tt.quality, tt.fragmented)
			if got != tt.want {
				t.Fatalf("selectFormat(%q, %t) = %q, want %q", tt.quality, tt.fragmented, got, tt.want)
			}
		})
	}
}

func TestNormalizeSubLangs(t *testing.T) {
	if got := normalizeSubLangs("english"); got != "en.*,en,-live_chat" {
		t.Fatalf("english normalize mismatch: %q", got)
	}
	if got := normalizeSubLangs("all"); got != "all,-live_chat" {
		t.Fatalf("all normalize mismatch: %q", got)
	}
	if got := normalizeSubLangs("de"); got != "de" {
		t.Fatalf("custom normalize mismatch: %q", got)
	}
}
