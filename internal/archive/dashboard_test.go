package archive

import "testing"

func TestEstimateTotalETA(t *testing.T) {
	got := estimateTotalETA(3_600_000_000, 0, 8.0)
	if got != "1h" {
		t.Fatalf("expected 1h, got %q", got)
	}

	got = estimateTotalETA(3_900_000_000, 0, 8.0)
	if got != "1h 5m" {
		t.Fatalf("expected 1h 5m, got %q", got)
	}

	got = estimateTotalETA(10_000_000, 0, 8.0)
	if got != "<1m" {
		t.Fatalf("expected <1m, got %q", got)
	}

	got = estimateTotalETA(1_000_000_000, 1_000_000_000, 8.0)
	if got != "0m" {
		t.Fatalf("expected 0m, got %q", got)
	}
}

func TestEstimateTotalETAInvalidInputs(t *testing.T) {
	if got := estimateTotalETA(0, 0, 8.0); got != "" {
		t.Fatalf("expected empty eta for missing size, got %q", got)
	}
	if got := estimateTotalETA(1_000_000_000, 0, 0); got != "" {
		t.Fatalf("expected empty eta for missing rate, got %q", got)
	}
}

