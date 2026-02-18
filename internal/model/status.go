package model

import "fmt"

const (
	StatusPending         = "pending"
	StatusRunning         = "running"
	StatusCompleted       = "completed"
	StatusFailedRetryable = "failed_retryable"
	StatusFailedPermanent = "failed_permanent"
	StatusSkippedPrivate  = "skipped_private"
)

var allowedTransitions = map[string]map[string]bool{
	"": {
		StatusPending:        true,
		StatusSkippedPrivate: true,
	},
	StatusPending: {
		StatusPending:         true,
		StatusRunning:         true,
		StatusFailedRetryable: true,
		StatusFailedPermanent: true,
		StatusSkippedPrivate:  true,
	},
	StatusRunning: {
		StatusRunning:         true,
		StatusCompleted:       true,
		StatusFailedRetryable: true,
		StatusFailedPermanent: true,
	},
	StatusCompleted: {
		StatusCompleted:       true,
		StatusPending:         true, // local media missing, needs re-download
		StatusFailedRetryable: true,
		StatusFailedPermanent: true,
	},
	StatusFailedRetryable: {
		StatusFailedRetryable: true,
		StatusRunning:         true,
		StatusPending:         true,
		StatusFailedPermanent: true,
		StatusSkippedPrivate:  true,
	},
	StatusFailedPermanent: {
		StatusFailedPermanent: true,
		StatusRunning:         true, // with explicit retry-permanent flow
		StatusPending:         true,
		StatusSkippedPrivate:  true,
	},
	StatusSkippedPrivate: {
		StatusSkippedPrivate: true,
		StatusPending:        true, // if a formerly private item becomes available
	},
}

func IsKnownStatus(status string) bool {
	_, ok := allowedTransitions[status]
	return ok
}

func CanTransition(from, to string) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

func TransitionJobStatus(job *Job, toStatus string, reason string) error {
	from := job.Status
	if !CanTransition(from, toStatus) {
		return fmt.Errorf("invalid job status transition: %q -> %q (job_id=%s video_id=%s)", from, toStatus, job.JobID, job.VideoID)
	}
	job.Status = toStatus
	job.Reason = reason
	return nil
}
