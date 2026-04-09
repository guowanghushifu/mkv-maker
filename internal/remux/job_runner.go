package remux

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type CommandRunner interface {
	Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error)
}

type JobRunner struct {
	runner                       CommandRunner
	renameOutput                 func(tempPath, finalPath string) error
	inspectIntermediateTrackJSON func(ctx context.Context, path string) ([]byte, error)
	runMakeMKVInfo               func(ctx context.Context, sourcePath string) ([]byte, error)
	runMakeMKVMKV                func(ctx context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error
	tempDir                      func() string
	cleanTempDir                 func(path string) error
	locateIntermediateMKV        func(tempDir string) (string, error)
	onCommandPreview             func(string)
	buildMKVMergeArgs            func(ctx context.Context, draft Draft) ([]string, error)
}

var makemkvconBinaryPath = "/opt/makemkv/bin/makemkvcon"

func NewJobRunner(runner CommandRunner) *JobRunner {
	if runner == nil {
		runner = MKVMergeRunner{}
	}
	jr := &JobRunner{
		runner:       runner,
		renameOutput: os.Rename,
		tempDir:      defaultMakeMKVTempDir,
		cleanTempDir: clearDirectoryContentsIfPresent,
	}
	jr.inspectIntermediateTrackJSON = jr.defaultInspectIntermediateTrackJSON
	jr.runMakeMKVInfo = jr.defaultRunMakeMKVInfo
	jr.runMakeMKVMKV = jr.defaultRunMakeMKVMKV
	jr.locateIntermediateMKV = defaultLocateIntermediateMKV
	jr.buildMKVMergeArgs = jr.defaultBuildMKVMergeArgs
	return jr
}

func (r *JobRunner) Execute(ctx context.Context, req StartRequest, onOutput func(string)) (string, bool, error) {
	if r == nil || r.runner == nil {
		return "", false, errors.New("runner is not configured")
	}

	draft, err := r.BuildExecutionDraft(req)
	if err != nil {
		return "", false, err
	}

	executionDraft := withTemporaryOutputPath(draft)
	tempPath := executionDraft.OutputPath
	if err := removeTemporaryOutput(tempPath); err != nil {
		return "", false, err
	}

	executionDraft, executionArgs, cleanupIntermediate, err := r.prepareExecutionDraft(ctx, executionDraft, onOutput)
	if cleanupIntermediate != nil {
		defer cleanupIntermediate()
	}
	if err != nil {
		return "", false, err
	}

	output, streamed, runErr := r.runCommand(ctx, executionDraft, executionArgs, onOutput)
	if runErr != nil {
		_ = removeTemporaryOutput(tempPath)
		return output, streamed, runErr
	}
	if err := r.finalizeOutput(tempPath, draft.OutputPath); err != nil {
		_ = removeTemporaryOutput(tempPath)
		return output, streamed, err
	}
	return output, streamed, nil
}

func (r *JobRunner) runCommand(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, bool, error) {
	var streamed atomic.Bool
	wrappedOutput := func(chunk string) {
		if strings.TrimSpace(chunk) == "" {
			return
		}
		streamed.Store(true)
		if onOutput != nil {
			onOutput(chunk)
		}
	}

	preview := FormatCommandPreview("mkvmerge", args)
	r.emitCommandPreview(preview)
	output, runErr := r.runner.Run(ctx, draft, args, wrappedOutput)
	return output, streamed.Load(), runErr
}

func (r *JobRunner) prepareExecutionDraft(ctx context.Context, draft Draft, onOutput func(string)) (Draft, []string, func(), error) {
	if strings.EqualFold(filepath.Ext(draft.SourcePath), ".mkv") {
		args, err := r.defaultBuildMKVMergeArgs(ctx, draft)
		return draft, args, nil, err
	}
	if r == nil {
		return draft, nil, nil, nil
	}

	workingDir := defaultMakeMKVTempDir()
	if r.tempDir != nil {
		workingDir = strings.TrimSpace(r.tempDir())
	}
	if workingDir == "" {
		workingDir = defaultMakeMKVTempDir()
	}
	cleanup := func() {
		if r.cleanTempDir != nil {
			_ = r.cleanTempDir(workingDir)
		}
	}
	if r.cleanTempDir != nil {
		if err := r.cleanTempDir(workingDir); err != nil {
			return Draft{}, nil, cleanup, err
		}
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		return Draft{}, nil, cleanup, err
	}

	if r.runMakeMKVInfo == nil {
		return Draft{}, nil, cleanup, errors.New("makemkv info runner is not configured")
	}
	robotOutput, err := r.runMakeMKVInfo(ctx, makeMKVSourcePath(draft))
	if err != nil {
		return Draft{}, nil, cleanup, err
	}
	playlistName := draft.Playlist
	if playlistName == "" {
		playlistName = filepath.Base(strings.TrimSpace(draft.SourcePath))
	}
	titleID := draft.MakeMKV.TitleID
	if titleID <= 0 {
		titleID, err = LookupMakeMKVTitleIDByPlaylist(robotOutput, playlistName)
		if err != nil {
			return Draft{}, nil, cleanup, err
		}
	}
	if r.runMakeMKVMKV == nil {
		return Draft{}, nil, cleanup, errors.New("makemkv mkv runner is not configured")
	}
	if err := r.runMakeMKVMKV(ctx, makeMKVSourcePath(draft), titleID, workingDir, onOutput); err != nil {
		return Draft{}, nil, cleanup, err
	}
	if r.locateIntermediateMKV == nil {
		return Draft{}, nil, cleanup, errors.New("intermediate mkv locator is not configured")
	}
	intermediatePath, err := r.locateIntermediateMKV(workingDir)
	if err != nil {
		return Draft{}, nil, cleanup, err
	}
	if r.inspectIntermediateTrackJSON == nil {
		return Draft{}, nil, cleanup, errors.New("intermediate track inspector is not configured")
	}
	identifyJSON, err := r.inspectIntermediateTrackJSON(ctx, intermediatePath)
	if err != nil {
		return Draft{}, nil, cleanup, err
	}

	secondPassDraft := draft
	secondPassDraft.SourcePath = intermediatePath
	audioSelectors, subtitleSelectors, err := BuildResolvedTrackSelectorsBySourceIndex(secondPassDraft, identifyJSON)
	if err != nil {
		return Draft{}, nil, cleanup, err
	}
	stageTwoArgs, err := BuildMKVMergeArgsWithResolvedSelectors(secondPassDraft, audioSelectors, subtitleSelectors)
	if err != nil {
		return Draft{}, nil, cleanup, err
	}
	r.emitCommandPreview(FormatCommandPreview("mkvmerge", stageTwoArgs))
	return secondPassDraft, stageTwoArgs, cleanup, nil
}

func (r *JobRunner) BuildExecutionDraft(req StartRequest) (Draft, error) {
	return buildExecutionDraft(req)
}

func (r *JobRunner) defaultBuildMKVMergeArgs(ctx context.Context, draft Draft) ([]string, error) {
	if r == nil || r.inspectIntermediateTrackJSON == nil {
		return BuildMKVMergeArgs(draft), nil
	}
	if !strings.EqualFold(filepath.Ext(draft.SourcePath), ".mkv") {
		return BuildMKVMergeArgs(draft), nil
	}
	if !hasSyntheticTrackIDs(draft) {
		return BuildMKVMergeArgs(draft), nil
	}

	identifyJSON, err := r.inspectIntermediateTrackJSON(ctx, draft.SourcePath)
	if err != nil {
		return nil, err
	}
	audioSelectors, subtitleSelectors, err := BuildResolvedTrackSelectorsBySourceIndex(draft, identifyJSON)
	if err != nil {
		return nil, err
	}
	return BuildMKVMergeArgsWithResolvedSelectors(draft, audioSelectors, subtitleSelectors)
}

func (r *JobRunner) emitCommandPreview(preview string) {
	if r == nil || r.onCommandPreview == nil {
		return
	}
	if strings.TrimSpace(preview) == "" {
		return
	}
	r.onCommandPreview(preview)
}

func hasSyntheticTrackIDs(draft Draft) bool {
	for _, track := range draft.Audio {
		if usesSyntheticTrackID(track.ID) {
			return true
		}
	}
	for _, track := range draft.Subtitles {
		if usesSyntheticTrackID(track.ID) {
			return true
		}
	}
	return false
}

func (r *JobRunner) defaultInspectIntermediateTrackJSON(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "mkvmerge", "-J", path)
	return cmd.Output()
}

func (r *JobRunner) defaultRunMakeMKVInfo(ctx context.Context, sourcePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, makemkvconBinaryPath, "info", makeMKVSourceArg(sourcePath), "--robot")
	cmd.Env = append(os.Environ(), "HOME=/config")
	return cmd.Output()
}

func (r *JobRunner) defaultRunMakeMKVMKV(ctx context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error {
	cmd := exec.CommandContext(
		ctx,
		makemkvconBinaryPath,
		"--messages=-null",
		"--progress=-stdout",
		"mkv",
		makeMKVSourceArg(sourcePath),
		strconv.Itoa(titleID),
		tempDir,
		"--profile=/config/nocore.mmcp.xml",
	)
	cmd.Env = append(os.Environ(), "HOME=/config")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	forward := func(chunk string) {
		if onOutput != nil {
			onOutput(chunk)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamOutput(stdout, forward)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderr, forward)
	}()
	waitErr := cmd.Wait()
	wg.Wait()
	return waitErr
}

func makeMKVSourceArg(path string) string {
	trimmed := strings.TrimSpace(path)
	if strings.EqualFold(filepath.Base(trimmed), "BDMV") {
		return "file:" + filepath.Dir(trimmed)
	}
	return "file:" + trimmed
}

func (r *JobRunner) CommandPreview(req StartRequest) (string, error) {
	draft, err := r.BuildExecutionDraft(req)
	if err != nil {
		return "", err
	}

	draft = withTemporaryOutputPath(draft)
	if !strings.EqualFold(filepath.Ext(draft.SourcePath), ".mkv") {
		workingDir := defaultMakeMKVTempDir()
		if r != nil && r.tempDir != nil {
			candidate := strings.TrimSpace(r.tempDir())
			if candidate != "" {
				workingDir = candidate
			}
		}
		titleID := draft.MakeMKV.TitleID
		if titleID <= 0 {
			return "", errors.New("makemkv title id is required for stage-one preview")
		}
		args := []string{
			"--messages=-null",
			"--progress=-stdout",
			"mkv",
			makeMKVSourceArg(makeMKVSourcePath(draft)),
			strconv.Itoa(titleID),
			workingDir,
			"--profile=/config/nocore.mmcp.xml",
		}
		return FormatCommandPreview("makemkvcon", args), nil
	}

	args, err := r.defaultBuildMKVMergeArgs(context.Background(), draft)
	if err != nil {
		return "", err
	}
	return FormatCommandPreview("mkvmerge", args), nil
}

func withTemporaryOutputPath(draft Draft) Draft {
	draft.OutputPath = temporaryOutputPath(draft.OutputPath)
	return draft
}

func temporaryOutputPath(finalPath string) string {
	return strings.TrimSpace(finalPath) + ".tmp"
}

func (r *JobRunner) finalizeOutput(tempPath, finalPath string) error {
	if r != nil && r.renameOutput != nil {
		return r.renameOutput(tempPath, finalPath)
	}
	return os.Rename(tempPath, finalPath)
}

func removeTemporaryOutput(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	err := os.Remove(trimmed)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func defaultMakeMKVTempDir() string {
	return "/remux_tmp"
}

func clearDirectoryContentsIfPresent(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	entries, err := os.ReadDir(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(trimmed, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func defaultLocateIntermediateMKV(tempDir string) (string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", err
	}
	var mkvPaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".mkv") {
			continue
		}
		mkvPaths = append(mkvPaths, filepath.Join(tempDir, entry.Name()))
	}
	switch len(mkvPaths) {
	case 1:
		return mkvPaths[0], nil
	case 0:
		return "", errors.New("intermediate mkv not found")
	default:
		return "", errors.New("multiple intermediate mkv files found")
	}
}

func makeMKVSourcePath(draft Draft) string {
	if strings.TrimSpace(draft.MakeMKVSourcePath) != "" {
		return strings.TrimSpace(draft.MakeMKVSourcePath)
	}
	return strings.TrimSpace(draft.SourcePath)
}

type MKVMergeRunner struct {
	Binary string
}

func (r MKVMergeRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	return r.runWithOutput(ctx, draft, args, onOutput)
}

func (r MKVMergeRunner) runWithOutput(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	_ = draft
	binary := resolveBinaryName(r.Binary)
	if len(args) == 0 {
		return "", errors.New("mkvmerge arguments are required")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var outputMu sync.Mutex
	var output strings.Builder
	appendChunk := func(chunk string) {
		if chunk == "" {
			return
		}
		outputMu.Lock()
		output.WriteString(chunk)
		outputMu.Unlock()
		if onOutput != nil {
			onOutput(chunk)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamOutput(stdout, appendChunk)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderr, appendChunk)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	return output.String(), waitErr
}

func resolveBinaryName(binary string) string {
	trimmed := strings.TrimSpace(binary)
	if trimmed == "" {
		return "mkvmerge"
	}
	return trimmed
}

func streamOutput(reader io.Reader, emit func(string)) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			emit(string(buf[:n]))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
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
		Title        string            `json:"title"`
		PlaylistName string            `json:"playlistName"`
		EnableDV     bool              `json:"dvMergeEnabled"`
		SegmentPaths []string          `json:"segmentPaths"`
		Video        VideoTrack        `json:"video"`
		Audio        []AudioTrack      `json:"audio"`
		Subtitles    []SubtitleTrack   `json:"subtitles"`
		MakeMKV      MakeMKVTitleCache `json:"makemkv"`
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
	makeMKVSourcePath := sourcePath
	if !strings.EqualFold(filepath.Ext(sourcePath), ".mkv") {
		sourcePath = resolvePlaylistPath(sourcePath, playlistName)
	} else {
		makeMKVSourcePath = ""
	}

	outputPath := strings.TrimSpace(payload.OutputPath)
	if outputPath == "" {
		outputPath = strings.TrimSpace(req.OutputPath)
	}
	if outputPath == "" {
		return Draft{}, errors.New("job payload is missing output path")
	}

	draft := Draft{
		Title:             strings.TrimSpace(payload.Draft.Title),
		Playlist:          playlistName,
		SourcePath:        sourcePath,
		MakeMKVSourcePath: makeMKVSourcePath,
		OutputPath:        outputPath,
		EnableDV:          payload.Draft.EnableDV,
		SegmentPaths:      payload.Draft.SegmentPaths,
		Video:             payload.Draft.Video,
		Audio:             payload.Draft.Audio,
		Subtitles:         payload.Draft.Subtitles,
		MakeMKV:           payload.Draft.MakeMKV,
	}
	if err := validateMakeMKVCache(draft, strings.TrimSpace(payload.Source.Type)); err != nil {
		return Draft{}, err
	}
	return draft, nil
}

func validateMakeMKVCache(draft Draft, sourceType string) error {
	if strings.EqualFold(filepath.Ext(strings.TrimSpace(draft.SourcePath)), ".mkv") {
		return nil
	}
	if sourceType != "" && !strings.EqualFold(sourceType, "bdmv") {
		return nil
	}
	if strings.TrimSpace(draft.MakeMKV.PlaylistName) == "" && draft.MakeMKV.TitleID == 0 && len(draft.MakeMKV.Audio) == 0 && len(draft.MakeMKV.Subtitles) == 0 {
		return errors.New("makemkv cache is required for bdmv remux")
	}
	if !strings.EqualFold(strings.TrimSpace(draft.MakeMKV.PlaylistName), strings.TrimSpace(draft.Playlist)) {
		return errors.New("makemkv cache playlist does not match draft playlist")
	}
	if draft.MakeMKV.TitleID < 0 {
		return errors.New("makemkv cache titleId is invalid")
	}
	return nil
}

func resolvePlaylistPath(sourcePath, playlistName string) string {
	if strings.EqualFold(filepath.Ext(sourcePath), ".MPLS") {
		return sourcePath
	}

	playlistDir := filepath.Join(sourcePath, "BDMV", "PLAYLIST")
	if strings.EqualFold(filepath.Base(sourcePath), "BDMV") {
		playlistDir = filepath.Join(sourcePath, "PLAYLIST")
	}

	exactPath := filepath.Join(playlistDir, playlistName)
	entries, err := os.ReadDir(playlistDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if entry.Name() == playlistName {
				return filepath.Join(playlistDir, entry.Name())
			}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.EqualFold(entry.Name(), playlistName) {
				return filepath.Join(playlistDir, entry.Name())
			}
		}
	}

	if _, err := os.Stat(exactPath); err == nil {
		return exactPath
	}

	return exactPath
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
