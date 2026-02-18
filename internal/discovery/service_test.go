package discovery

import (
	"testing"

	"yt-vod-manager/internal/model"
)

func TestMergeJobs_PreservesStateAndAddsNew(t *testing.T) {
	existing := []model.Job{
		{
			JobID:    "src:1:a",
			Index:    1,
			VideoID:  "a",
			VideoURL: "https://www.youtube.com/watch?v=a",
			Title:    "A",
			Status:   model.StatusCompleted,
		},
		{
			JobID:    "src:2:b",
			Index:    2,
			VideoID:  "b",
			VideoURL: "https://www.youtube.com/watch?v=b",
			Title:    "B",
			Status:   model.StatusFailedRetryable,
		},
	}

	src := sourceManifest{
		ID:    "src",
		Title: "source",
		Entries: []sourceEntry{
			{ID: "c", Title: "C", VideoURL: "https://www.youtube.com/watch?v=c"},
			{ID: "a", Title: "A2", VideoURL: "https://www.youtube.com/watch?v=a"},
			{ID: "b", Title: "B2", VideoURL: "https://www.youtube.com/watch?v=b"},
		},
	}

	jobs, added := mergeJobs(existing, src)
	if added != 1 {
		t.Fatalf("expected 1 added job, got %d", added)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	if jobs[0].VideoID != "c" || jobs[0].Status != model.StatusPending {
		t.Fatalf("expected first job to be new pending c, got %+v", jobs[0])
	}
	if jobs[1].VideoID != "a" || jobs[1].Status != model.StatusCompleted {
		t.Fatalf("expected completed state preserved for a, got %+v", jobs[1])
	}
	if jobs[2].VideoID != "b" || jobs[2].Status != model.StatusFailedRetryable {
		t.Fatalf("expected retryable state preserved for b, got %+v", jobs[2])
	}
}

func TestMergeJobs_ConvertsRunningToRetryable(t *testing.T) {
	existing := []model.Job{
		{
			JobID:    "src:1:a",
			Index:    1,
			VideoID:  "a",
			VideoURL: "https://www.youtube.com/watch?v=a",
			Title:    "A",
			Status:   model.StatusRunning,
		},
	}
	src := sourceManifest{
		ID:      "src",
		Title:   "source",
		Entries: []sourceEntry{{ID: "a", Title: "A", VideoURL: "https://www.youtube.com/watch?v=a"}},
	}

	jobs, added := mergeJobs(existing, src)
	if added != 0 {
		t.Fatalf("expected no added jobs, got %d", added)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != model.StatusFailedRetryable {
		t.Fatalf("expected running -> failed_retryable on merge, got %s", jobs[0].Status)
	}
	if jobs[0].Reason != "interrupted_previous_run" {
		t.Fatalf("expected interrupted reason, got %q", jobs[0].Reason)
	}
}
