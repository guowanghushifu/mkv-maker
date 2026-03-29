package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type JobStore interface {
	MarkRunningJobsInterrupted() error
}

type ExecutionJob struct {
	ID          string
	PayloadJSON string
	OutputPath  string
	LogPath     string
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
var ErrJobAlreadyRunning = errors.New("job already running")

type SQLiteJobStore struct {
	db      *sql.DB
	logsDir string
}

type persistedJobMetadata struct {
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
	PlaylistName string `json:"playlistName"`
}

type persistedJobPayload struct {
	Source struct {
		Name string `json:"name"`
	} `json:"source"`
	BDInfo struct {
		PlaylistName string `json:"playlistName"`
	} `json:"bdinfo"`
	Draft struct {
		PlaylistName string `json:"playlistName"`
	} `json:"draft"`
	OutputFilename string `json:"outputFilename"`
	OutputPath     string `json:"outputPath"`
}

func NewSQLiteJobStore(db *sql.DB, logsDir string) *SQLiteJobStore {
	return &SQLiteJobStore{db: db, logsDir: strings.TrimSpace(logsDir)}
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
	})
	if err != nil {
		return APIJob{}, err
	}
	draftJSON := strings.TrimSpace(input.PayloadJSON)
	if draftJSON == "" {
		draftJSON = string(metadataJSON)
	}

	logPath := ""
	if s.logsDir != "" {
		if err := os.MkdirAll(s.logsDir, 0o755); err != nil {
			return APIJob{}, err
		}
		logPath = filepath.Join(s.logsDir, id+".log")
		if err := os.WriteFile(logPath, []byte(buildInitialJobLog(input.PlaylistName, input.OutputPath)), 0o644); err != nil {
			return APIJob{}, err
		}
	}

	if _, err := s.db.Exec(
		`insert into jobs(id, status, draft_json, output_path, log_path) values(?, ?, ?, ?, ?)`,
		id,
		"queued",
		draftJSON,
		strings.TrimSpace(input.OutputPath),
		logPath,
	); err != nil {
		return APIJob{}, err
	}

	return s.GetJob(id)
}

func (s *SQLiteJobStore) CreateRunningJob(input CreateJobInput) (APIJob, error) {
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
	})
	if err != nil {
		return APIJob{}, err
	}
	draftJSON := strings.TrimSpace(input.PayloadJSON)
	if draftJSON == "" {
		draftJSON = string(metadataJSON)
	}

	logPath := ""
	if s.logsDir != "" {
		if err := os.MkdirAll(s.logsDir, 0o755); err != nil {
			return APIJob{}, err
		}
		logPath = filepath.Join(s.logsDir, id+".log")
		if err := os.WriteFile(logPath, []byte(buildInitialJobLog(input.PlaylistName, input.OutputPath)), 0o644); err != nil {
			return APIJob{}, err
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return APIJob{}, err
	}
	defer tx.Rollback()

	var runningCount int
	if err := tx.QueryRow(`select count(1) from jobs where status = 'running'`).Scan(&runningCount); err != nil {
		return APIJob{}, err
	}
	if runningCount > 0 {
		return APIJob{}, ErrJobAlreadyRunning
	}

	if _, err := tx.Exec(
		`insert into jobs(id, status, draft_json, output_path, log_path, started_at) values(?, ?, ?, ?, ?, current_timestamp)`,
		id,
		"running",
		draftJSON,
		strings.TrimSpace(input.OutputPath),
		logPath,
	); err != nil {
		return APIJob{}, err
	}
	if err := tx.Commit(); err != nil {
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
	if s == nil || s.db == nil {
		return "", errors.New("job store is not configured")
	}

	var logPath string
	err := s.db.QueryRow(`select log_path from jobs where id = ?`, strings.TrimSpace(id)).Scan(&logPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrJobNotFound
		}
		return "", err
	}
	logPath = strings.TrimSpace(logPath)
	if logPath != "" {
		content, err := os.ReadFile(logPath)
		if err == nil {
			return string(content), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	job, err := s.GetJob(id)
	if err != nil {
		return "", err
	}
	return buildInitialJobLog(job.PlaylistName, job.OutputPath), nil
}

func (s *SQLiteJobStore) GetCurrentJob() (APIJob, error) {
	if s == nil || s.db == nil {
		return APIJob{}, errors.New("job store is not configured")
	}

	var (
		id         string
		status     string
		draftJSON  string
		outputPath string
		errorText  string
		createdAt  string
	)
	err := s.db.QueryRow(`
		select id, status, draft_json, output_path, error_text, created_at
		from jobs
		order by case when status = 'running' then 0 else 1 end asc, datetime(created_at) desc, id desc
		limit 1
	`).Scan(&id, &status, &draftJSON, &outputPath, &errorText, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIJob{}, ErrJobNotFound
		}
		return APIJob{}, err
	}

	return buildAPIJob(id, status, draftJSON, outputPath, errorText, createdAt), nil
}

func (s *SQLiteJobStore) GetCurrentJobLog() (string, error) {
	job, err := s.GetCurrentJob()
	if err != nil {
		return "", err
	}
	return s.GetJobLog(job.ID)
}

func (s *SQLiteJobStore) ClaimNextQueuedJob() (ExecutionJob, bool, error) {
	if s == nil || s.db == nil {
		return ExecutionJob{}, false, errors.New("job store is not configured")
	}

	var job ExecutionJob
	err := s.db.QueryRow(`
		select id, draft_json, output_path, log_path
		from jobs
		where status = 'queued'
		order by datetime(created_at) asc, id asc
		limit 1
	`).Scan(&job.ID, &job.PayloadJSON, &job.OutputPath, &job.LogPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExecutionJob{}, false, nil
		}
		return ExecutionJob{}, false, err
	}

	job.ID = strings.TrimSpace(job.ID)
	job.PayloadJSON = strings.TrimSpace(job.PayloadJSON)
	job.OutputPath = strings.TrimSpace(job.OutputPath)
	job.LogPath = strings.TrimSpace(job.LogPath)
	return job, true, nil
}

func (s *SQLiteJobStore) MarkJobRunning(id string) error {
	return s.updateStatus(strings.TrimSpace(id), "running", "", "queued")
}

func (s *SQLiteJobStore) MarkJobCompleted(id string) error {
	return s.updateStatus(strings.TrimSpace(id), "succeeded", "", "")
}

func (s *SQLiteJobStore) MarkJobFailed(id, message string) error {
	return s.updateStatus(strings.TrimSpace(id), "failed", strings.TrimSpace(message), "")
}

func (s *SQLiteJobStore) AppendJobLog(id, content string) error {
	if s == nil || s.db == nil {
		return errors.New("job store is not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrJobNotFound
	}
	if content == "" {
		return nil
	}

	logPath, err := s.ensureLogPath(id)
	if err != nil {
		return err
	}
	if logPath == "" {
		return nil
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteJobStore) GetJobPayloadJSON(id string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("job store is not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrJobNotFound
	}

	var payloadJSON string
	err := s.db.QueryRow(`select draft_json from jobs where id = ?`, id).Scan(&payloadJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrJobNotFound
		}
		return "", err
	}
	return strings.TrimSpace(payloadJSON), nil
}

func (s *SQLiteJobStore) GetExecutionJob(id string) (ExecutionJob, error) {
	if s == nil || s.db == nil {
		return ExecutionJob{}, errors.New("job store is not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ExecutionJob{}, ErrJobNotFound
	}

	var job ExecutionJob
	err := s.db.QueryRow(`
		select id, draft_json, output_path, log_path
		from jobs
		where id = ?
	`, id).Scan(&job.ID, &job.PayloadJSON, &job.OutputPath, &job.LogPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExecutionJob{}, ErrJobNotFound
		}
		return ExecutionJob{}, err
	}
	job.ID = strings.TrimSpace(job.ID)
	job.PayloadJSON = strings.TrimSpace(job.PayloadJSON)
	job.OutputPath = strings.TrimSpace(job.OutputPath)
	job.LogPath = strings.TrimSpace(job.LogPath)
	return job, nil
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

func (s *SQLiteJobStore) MarkRunningJobsFailed(message string) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`
		update jobs
		set status = 'failed',
		    error_text = ?,
		    finished_at = current_timestamp
		where status = 'running'
	`, strings.TrimSpace(message))
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

func (s *SQLiteJobStore) ensureLogPath(id string) (string, error) {
	var logPath string
	err := s.db.QueryRow(`select log_path from jobs where id = ?`, id).Scan(&logPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrJobNotFound
		}
		return "", err
	}
	logPath = strings.TrimSpace(logPath)
	if logPath != "" {
		return logPath, nil
	}

	if s.logsDir == "" {
		return "", nil
	}
	if err := os.MkdirAll(s.logsDir, 0o755); err != nil {
		return "", err
	}
	logPath = filepath.Join(s.logsDir, id+".log")
	if _, err := s.db.Exec(`update jobs set log_path = ? where id = ?`, logPath, id); err != nil {
		return "", err
	}
	return logPath, nil
}

func (s *SQLiteJobStore) updateStatus(id, status, errorText, requireCurrentStatus string) error {
	if s == nil || s.db == nil {
		return errors.New("job store is not configured")
	}
	if id == "" {
		return ErrJobNotFound
	}

	var (
		res sql.Result
		err error
	)
	switch {
	case status == "running":
		if requireCurrentStatus == "" {
			requireCurrentStatus = "queued"
		}
		res, err = s.db.Exec(`
			update jobs
			set status = ?,
			    error_text = '',
			    started_at = coalesce(started_at, current_timestamp),
			    finished_at = null
			where id = ? and status = ?
		`, status, id, requireCurrentStatus)
	case errorText == "":
		res, err = s.db.Exec(`
			update jobs
			set status = ?,
			    error_text = '',
			    finished_at = current_timestamp
			where id = ?
		`, status, id)
	default:
		res, err = s.db.Exec(`
			update jobs
			set status = ?,
			    error_text = ?,
			    finished_at = current_timestamp
			where id = ?
		`, status, errorText, id)
	}
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrJobNotFound
	}
	return nil
}

func decodeJobMetadata(value string) persistedJobMetadata {
	var metadata persistedJobMetadata
	if err := json.Unmarshal([]byte(value), &metadata); err == nil {
		if metadata.SourceName != "" || metadata.OutputName != "" || metadata.PlaylistName != "" {
			return metadata
		}
	}

	var payload persistedJobPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return persistedJobMetadata{}
	}

	playlistName := strings.TrimSpace(payload.BDInfo.PlaylistName)
	if playlistName == "" {
		playlistName = strings.TrimSpace(payload.Draft.PlaylistName)
	}
	return persistedJobMetadata{
		SourceName:   strings.TrimSpace(payload.Source.Name),
		OutputName:   strings.TrimSpace(payload.OutputFilename),
		PlaylistName: playlistName,
	}
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

func buildInitialJobLog(playlistName, outputPath string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	lines := []string{"[" + timestamp + "] remux started"}
	if strings.TrimSpace(playlistName) != "" {
		lines = append(lines, "Resolving playlist "+strings.TrimSpace(playlistName))
	}
	if strings.TrimSpace(outputPath) != "" {
		lines = append(lines, "Preparing output "+strings.TrimSpace(outputPath))
	}
	return strings.Join(lines, "\n")
}
