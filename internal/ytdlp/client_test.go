package ytdlp

import "testing"

func TestFormatRateLimitMBps(t *testing.T) {
	if got := formatRateLimitMBps(10); got != "10M" {
		t.Fatalf("unexpected rate format: got %q want %q", got, "10M")
	}
	if got := formatRateLimitMBps(2.5); got != "2.5M" {
		t.Fatalf("unexpected rate format: got %q want %q", got, "2.5M")
	}
}
