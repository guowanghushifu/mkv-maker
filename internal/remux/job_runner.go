package remux

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommandRunner interface {
	Run(ctx context.Context, draft Draft) (string, error)
}

type JobRunner struct {
	runner CommandRunner
}

func NewJobRunner(runner CommandRunner) *JobRunner {
	if runner == nil {
		runner = MKVMergeRunner{}
	}
	return &JobRunner{runner: runner}
}

func (r *JobRunner) Execute(ctx context.Context, req StartRequest) (string, error) {
	if r == nil || r.runner == nil {
		return "", errors.New("runner is not configured")
	}

	draft, err := r.BuildExecutionDraft(req)
	if err != nil {
		return "", err
	}
	return r.runner.Run(ctx, draft)
}

func (r *JobRunner) BuildExecutionDraft(req StartRequest) (Draft, error) {
	return buildExecutionDraft(req)
}

func (r *JobRunner) CommandPreview(req StartRequest) (string, error) {
	draft, err := r.BuildExecutionDraft(req)
	if err != nil {
		return "", err
	}

	args := BuildMKVMergeArgs(draft)
	return FormatCommandPreview(r.commandBinary(), args), nil
}

type MKVMergeRunner struct {
	Binary string
}

func (r MKVMergeRunner) Run(ctx context.Context, draft Draft) (string, error) {
	binary := resolveBinaryName(r.Binary)

	args := BuildMKVMergeArgs(draft)
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (r *JobRunner) commandBinary() string {
	if r == nil || r.runner == nil {
		return "mkvmerge"
	}
	switch typed := r.runner.(type) {
	case MKVMergeRunner:
		return resolveBinaryName(typed.Binary)
	case *MKVMergeRunner:
		return resolveBinaryName(typed.Binary)
	default:
		return "mkvmerge"
	}
}

func resolveBinaryName(binary string) string {
	trimmed := strings.TrimSpace(binary)
	if trimmed == "" {
		return "mkvmerge"
	}
	return trimmed
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
		Title        string          `json:"title"`
		PlaylistName string          `json:"playlistName"`
		EnableDV     bool            `json:"dvMergeEnabled"`
		SegmentPaths []string        `json:"segmentPaths"`
		Video        VideoTrack      `json:"video"`
		Audio        []AudioTrack    `json:"audio"`
		Subtitles    []SubtitleTrack `json:"subtitles"`
	} `json:"draft"`
	OutputPath string `json:"outputPath"`
}

func buildExecutionDraft(req StartRequest) (Draft, error) {
	payloadJSON := strings.TrimSpace(req.PayloadJSON)
	if payloadJSON == "" {
		return Draft{}, errors.New("job payload is empty")
	}

	var payload executionPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return Draft{}, err
	}

	if sourceType := strings.TrimSpace(payload.Source.Type); sourceType != "" && !strings.EqualFold(sourceType, "bdmv") {
		return Draft{}, errors.New("only bdmv source payloads are supported")
	}

	playlistName := strings.ToUpper(strings.TrimSpace(payload.BDInfo.PlaylistName))
	if playlistName == "" {
		playlistName = strings.ToUpper(strings.TrimSpace(payload.Draft.PlaylistName))
	}
	if playlistName == "" {
		playlistName = strings.ToUpper(strings.TrimSpace(req.PlaylistName))
	}
	if playlistName == "" {
		return Draft{}, errors.New("job payload is missing playlist name")
	}

	sourcePath := strings.TrimSpace(payload.Source.Path)
	if sourcePath == "" {
		return Draft{}, errors.New("job payload is missing source path")
	}
	sourcePath = resolvePlaylistPath(sourcePath, playlistName)

	outputPath := strings.TrimSpace(payload.OutputPath)
	if outputPath == "" {
		outputPath = strings.TrimSpace(req.OutputPath)
	}
	if outputPath == "" {
		return Draft{}, errors.New("job payload is missing output path")
	}

	return Draft{
		Title:        strings.TrimSpace(payload.Draft.Title),
		Playlist:     playlistName,
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
