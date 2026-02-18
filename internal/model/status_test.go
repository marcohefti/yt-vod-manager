package model

import "testing"

func TestCanTransition_AllowsExpectedPaths(t *testing.T) {
	cases := []struct {
		from string
		to   string
	}{
		{"", StatusPending},
		{StatusPending, StatusRunning},
		{StatusRunning, StatusCompleted},
		{StatusRunning, StatusFailedRetryable},
		{StatusFailedRetryable, StatusRunning},
		{StatusCompleted, StatusPending},
		{StatusSkippedPrivate, StatusPending},
		{StatusFailedPermanent, StatusRunning},
	}

	for _, tc := range cases {
		if !CanTransition(tc.from, tc.to) {
			t.Fatalf("expected transition %q -> %q to be allowed", tc.from, tc.to)
		}
	}
}

func TestCanTransition_RejectsInvalidPaths(t *testing.T) {
	cases := []struct {
		from string
		to   string
	}{
		{StatusPending, StatusCompleted},
		{StatusSkippedPrivate, StatusRunning},
		{StatusCompleted, StatusRunning},
		{"not_a_state", StatusPending},
	}

	for _, tc := range cases {
		if CanTransition(tc.from, tc.to) {
			t.Fatalf("expected transition %q -> %q to be rejected", tc.from, tc.to)
		}
	}
}

func TestTransitionJobStatus_BlocksIllegalTransition(t *testing.T) {
	job := Job{
		JobID:   "job-1",
		VideoID: "vid-1",
		Status:  StatusPending,
	}

	if err := TransitionJobStatus(&job, StatusCompleted, ""); err == nil {
		t.Fatalf("expected illegal transition error")
	}
}
