package queue

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/remux"
	"github.com/guowanghushifu/mkv-maker/internal/store"
)

type ExecutionStore interface {
	ClaimNextQueuedJob() (store.ExecutionJob, bool, error)
	MarkJobRunning(id string) error
	MarkJobCompleted(id string) error
	MarkJobFailed(id, message string) error
	AppendJobLog(id, content string) error
}

type CommandRunner interface {
	Run(ctx context.Context, draft remux.Draft) (string, error)
}

type Executor struct {
	store  ExecutionStore
	runner CommandRunner
}

func NewExecutor(jobStore ExecutionStore, runner CommandRunner) *Executor {
	if runner == nil {
		runner = MKVMergeRunner{}
	}
	return &Executor{
		store:  jobStore,
		runner: runner,
	}
}

func (e *Executor) ExecuteNext(ctx context.Context) (bool, error) {
	if e == nil || e.store == nil {
		return false, nil
	}

	job, ok, err := e.store.ClaimNextQueuedJob()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	if err := e.store.MarkJobRunning(job.ID); err != nil {
		return true, err
	}
	if err := e.store.AppendJobLog(job.ID, logLine("running")); err != nil {
		_ = e.store.MarkJobFailed(job.ID, "failed to append running log: "+err.Error())
		return true, nil
	}

	draft, err := buildExecutionDraft(job)
	if err != nil {
		_ = e.store.AppendJobLog(job.ID, logLine("failed to reconstruct draft: "+err.Error()))
		_ = e.store.MarkJobFailed(job.ID, err.Error())
		return true, nil
	}

	output, runErr := e.runner.Run(ctx, draft)
	if output != "" {
		if err := e.store.AppendJobLog(job.ID, normalizeLogChunk(output)); err != nil {
			_ = e.store.MarkJobFailed(job.ID, "failed to append command output: "+err.Error())
			return true, nil
		}
	}

	if runErr != nil {
		message := normalizeRunnerError(runErr)
		_ = e.store.AppendJobLog(job.ID, logLine(message))
		_ = e.store.MarkJobFailed(job.ID, message)
		return true, nil
	}

	if err := e.store.AppendJobLog(job.ID, logLine("completed")); err != nil {
		_ = e.store.MarkJobFailed(job.ID, "failed to append completion log: "+err.Error())
		return true, nil
	}
	if err := e.store.MarkJobCompleted(job.ID); err != nil {
		return true, err
	}

	return true, nil
}

type MKVMergeRunner struct {
	Binary string
}

func (r MKVMergeRunner) Run(ctx context.Context, draft remux.Draft) (string, error) {
	binary := strings.TrimSpace(r.Binary)
	if binary == "" {
		binary = "mkvmerge"
	}

	args := remux.BuildMKVMergeArgs(draft)
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type executionPayload struct {
	Source struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"source"`
	BDInfo struct {
		PlaylistName string `json:"playlistName"`
	} `json:"bdinfo"`
	Draft struct {
		Title        string                `json:"title"`
		PlaylistName string                `json:"playlistName"`
		EnableDV     bool                  `json:"dvMergeEnabled"`
		SegmentPaths []string              `json:"segmentPaths"`
		Video        remux.VideoTrack      `json:"video"`
		Audio        []remux.AudioTrack    `json:"audio"`
		Subtitles    []remux.SubtitleTrack `json:"subtitles"`
	} `json:"draft"`
	OutputPath string `json:"outputPath"`
}

func buildExecutionDraft(job store.ExecutionJob) (remux.Draft, error) {
	payloadJSON := strings.TrimSpace(job.PayloadJSON)
	if payloadJSON == "" {
		return remux.Draft{}, errors.New("job payload is empty")
	}

	var payload executionPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return remux.Draft{}, err
	}

	if sourceType := strings.TrimSpace(payload.Source.Type); sourceType != "" && !strings.EqualFold(sourceType, "bdmv") {
		return remux.Draft{}, errors.New("only bdmv source payloads are supported")
	}

	playlistName := strings.ToUpper(strings.TrimSpace(payload.BDInfo.PlaylistName))
	if playlistName == "" {
		playlistName = strings.ToUpper(strings.TrimSpace(payload.Draft.PlaylistName))
	}
	if playlistName == "" {
		return remux.Draft{}, errors.New("job payload is missing playlist name")
	}

	sourcePath := strings.TrimSpace(payload.Source.Path)
	if sourcePath == "" {
		return remux.Draft{}, errors.New("job payload is missing source path")
	}
	sourcePath = resolvePlaylistPath(sourcePath, playlistName)

	outputPath := strings.TrimSpace(payload.OutputPath)
	if outputPath == "" {
		outputPath = strings.TrimSpace(job.OutputPath)
	}
	if outputPath == "" {
		return remux.Draft{}, errors.New("job payload is missing output path")
	}

	return remux.Draft{
		Title:        strings.TrimSpace(payload.Draft.Title),
		SourcePath:   sourcePath,
		OutputPath:   outputPath,
		EnableDV:     payload.Draft.EnableDV,
		SegmentPaths: payload.Draft.SegmentPaths,
		Video:        payload.Draft.Video,
		Audio:        payload.Draft.Audio,
		Subtitles:    payload.Draft.Subtitles,
	}, nil
}

func resolvePlaylistPath(sourcePath, playlistName string) string {
	if strings.EqualFold(filepath.Ext(sourcePath), ".MPLS") {
		return sourcePath
	}
	if strings.EqualFold(filepath.Base(sourcePath), "BDMV") {
		return filepath.Join(sourcePath, "PLAYLIST", playlistName)
	}
	return filepath.Join(sourcePath, "BDMV", "PLAYLIST", playlistName)
}

func normalizeRunnerError(err error) string {
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return "mkvmerge executable not found in PATH"
	}
	return strings.TrimSpace(err.Error())
}

func logLine(message string) string {
	return "[" + time.Now().UTC().Format(time.RFC3339) + "] " + strings.TrimSpace(message) + "\n"
}

func normalizeLogChunk(chunk string) string {
	if strings.HasSuffix(chunk, "\n") {
		return chunk
	}
	return chunk + "\n"
}
