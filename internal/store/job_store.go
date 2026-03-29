package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type JobStore interface {
	MarkRunningJobsInterrupted() error
}

type APIJob struct {
	ID           string `json:"id"`
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
	OutputPath   string `json:"outputPath,omitempty"`
	PlaylistName string `json:"playlistName"`
	CreatedAt    string `json:"createdAt"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
}

type CreateJobInput struct {
	SourceName   string
	OutputName   string
	OutputPath   string
	PlaylistName string
	PayloadJSON  string
}

type JobsRepository interface {
	CreateQueuedJob(input CreateJobInput) (APIJob, error)
	ListJobs() ([]APIJob, error)
	GetJob(id string) (APIJob, error)
	GetJobLog(id string) (string, error)
}

var ErrJobNotFound = errors.New("job not found")

type SQLiteJobStore struct {
	db *sql.DB
}

type persistedJobMetadata struct {
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
	PlaylistName string `json:"playlistName"`
	PayloadJSON  string `json:"payloadJson,omitempty"`
}

func NewSQLiteJobStore(db *sql.DB) *SQLiteJobStore {
	return &SQLiteJobStore{db: db}
}

func (s *SQLiteJobStore) CreateQueuedJob(input CreateJobInput) (APIJob, error) {
	if s == nil || s.db == nil {
		return APIJob{}, errors.New("job store is not configured")
	}

	id, err := generateJobID()
	if err != nil {
		return APIJob{}, err
	}

	metadataJSON, err := json.Marshal(persistedJobMetadata{
		SourceName:   strings.TrimSpace(input.SourceName),
		OutputName:   strings.TrimSpace(input.OutputName),
		PlaylistName: strings.TrimSpace(input.PlaylistName),
		PayloadJSON:  strings.TrimSpace(input.PayloadJSON),
	})
	if err != nil {
		return APIJob{}, err
	}

	if _, err := s.db.Exec(
		`insert into jobs(id, status, draft_json, output_path) values(?, ?, ?, ?)`,
		id,
		"queued",
		string(metadataJSON),
		strings.TrimSpace(input.OutputPath),
	); err != nil {
		return APIJob{}, err
	}

	return s.GetJob(id)
}

func (s *SQLiteJobStore) ListJobs() ([]APIJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("job store is not configured")
	}

	rows, err := s.db.Query(`
		select id, status, draft_json, output_path, error_text, created_at
		from jobs
		order by datetime(created_at) desc, id desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]APIJob, 0, 16)
	for rows.Next() {
		job, err := scanAPIJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *SQLiteJobStore) GetJob(id string) (APIJob, error) {
	if s == nil || s.db == nil {
		return APIJob{}, errors.New("job store is not configured")
	}
	var (
		jobID      string
		status     string
		draftJSON  string
		outputPath string
		errorText  string
		createdAt  string
	)
	err := s.db.QueryRow(`
		select id, status, draft_json, output_path, error_text, created_at
		from jobs
		where id = ?
	`, strings.TrimSpace(id)).Scan(&jobID, &status, &draftJSON, &outputPath, &errorText, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIJob{}, ErrJobNotFound
		}
		return APIJob{}, err
	}
	return buildAPIJob(jobID, status, draftJSON, outputPath, errorText, createdAt), nil
}

func (s *SQLiteJobStore) GetJobLog(id string) (string, error) {
	job, err := s.GetJob(id)
	if err != nil {
		return "", err
	}
	timestamp := strings.TrimSpace(job.CreatedAt)
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	lines := []string{
		"[" + timestamp + "] " + strings.TrimSpace(job.Status),
	}
	if strings.TrimSpace(job.PlaylistName) != "" {
		lines = append(lines, "Resolving playlist "+job.PlaylistName)
	}
	if strings.TrimSpace(job.OutputPath) != "" {
		lines = append(lines, "Preparing output "+job.OutputPath)
	}
	if strings.TrimSpace(job.Message) != "" {
		lines = append(lines, job.Message)
	}
	return strings.Join(lines, "\n"), nil
}

func (s *SQLiteJobStore) MarkRunningJobsInterrupted() error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`
		update jobs
		set status = 'interrupted',
		    error_text = case
		        when trim(error_text) = '' then 'interrupted by startup recovery'
		        else error_text
		    end,
		    finished_at = current_timestamp
		where status = 'running'
	`)
	return err
}

func scanAPIJob(rows *sql.Rows) (APIJob, error) {
	var (
		id         string
		status     string
		draftJSON  string
		outputPath string
		errorText  string
		createdAt  string
	)
	if err := rows.Scan(&id, &status, &draftJSON, &outputPath, &errorText, &createdAt); err != nil {
		return APIJob{}, err
	}
	return buildAPIJob(id, status, draftJSON, outputPath, errorText, createdAt), nil
}

func buildAPIJob(id, status, draftJSON, outputPath, errorText, createdAt string) APIJob {
	meta := decodeJobMetadata(draftJSON)
	return APIJob{
		ID:           id,
		SourceName:   fallback(meta.SourceName, "Unknown Source"),
		OutputName:   fallback(meta.OutputName, "pending.mkv"),
		OutputPath:   strings.TrimSpace(outputPath),
		PlaylistName: fallback(meta.PlaylistName, "unknown"),
		CreatedAt:    normalizeCreatedAt(createdAt),
		Status:       fallback(status, "queued"),
		Message:      strings.TrimSpace(errorText),
	}
}

func decodeJobMetadata(value string) persistedJobMetadata {
	var metadata persistedJobMetadata
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return persistedJobMetadata{}
	}
	return metadata
}

func normalizeCreatedAt(createdAt string) string {
	createdAt = strings.TrimSpace(createdAt)
	if createdAt == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, createdAt); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return createdAt
}

func fallback(value, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func generateJobID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "job-" + hex.EncodeToString(buf), nil
}
