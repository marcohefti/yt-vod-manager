package model

// JobsManifest is the canonical per-run job state file.
type JobsManifest struct {
	SchemaVersion   int    `json:"schema_version"`
	GeneratedAt     string `json:"generated_at"`
	RunID           string `json:"run_id"`
	Profile         string `json:"profile"`
	SourceURL       string `json:"source_url"`
	SourceID        string `json:"source_id,omitempty"`
	SourceTitle     string `json:"source_title,omitempty"`
	SourceType      string `json:"source_type,omitempty"`
	PlaylistID      string `json:"playlist_id"`
	PlaylistTitle   string `json:"playlist_title"`
	Total           int    `json:"total"`
	Pending         int    `json:"pending"`
	Running         int    `json:"running"`
	Completed       int    `json:"completed"`
	FailedRetryable int    `json:"failed_retryable"`
	FailedPermanent int    `json:"failed_permanent"`
	SkippedPrivate  int    `json:"skipped_private"`
	Jobs            []Job  `json:"jobs"`
}

type Job struct {
	JobID         string `json:"job_id"`
	Index         int    `json:"index"`
	VideoID       string `json:"video_id"`
	VideoURL      string `json:"video_url"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	Attempts      int    `json:"attempts,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	LastAttemptAt string `json:"last_attempt_at,omitempty"`
	CompletedAt   string `json:"completed_at,omitempty"`
}
