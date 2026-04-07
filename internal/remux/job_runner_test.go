package remux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fileWritingRunner struct {
	run func(ctx context.Context, draft Draft, onOutput func(string)) (string, error)
}

func (r fileWritingRunner) Run(ctx context.Context, draft Draft, onOutput func(string)) (string, error) {
	return r.run(ctx, draft, onOutput)
}

func TestBuildExecutionDraftUsesExistingPlaylistPathCaseInsensitive(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	playlistPath := filepath.Join(sourcePath, "PLAYLIST", "00801.mpls")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validPayloadJSON("Disc", sourcePath, "00801.MPLS", "/remux/Disc.mkv"),
	}

	draft, err := runner.BuildExecutionDraft(req)
	if err != nil {
		t.Fatalf("BuildExecutionDraft returned error: %v", err)
	}
	if draft.SourcePath != playlistPath {
		t.Fatalf("expected SourcePath %q, got %q", playlistPath, draft.SourcePath)
	}

	preview, err := runner.CommandPreview(req)
	if err != nil {
		t.Fatalf("CommandPreview returned error: %v", err)
	}
	if !strings.Contains(preview, playlistPath) {
		t.Fatalf("expected preview to contain %q, got %q", playlistPath, preview)
	}
}

func TestBuildExecutionDraftPreservesIntermediateMKVSourcePath(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/tmp/intermediate.mkv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{"playlistName":"00801.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	draft, err := runner.BuildExecutionDraft(req)
	if err != nil {
		t.Fatalf("BuildExecutionDraft returned error: %v", err)
	}
	if draft.SourcePath != "/tmp/intermediate.mkv" {
		t.Fatalf("expected intermediate mkv source path to be preserved, got %q", draft.SourcePath)
	}
}

func TestMakeMKVSourceArgumentDoesNotShellQuotePath(t *testing.T) {
	arg := makeMKVSourceArg("/bd input/Disc")
	if arg != "file:/bd input/Disc" {
		t.Fatalf("expected unquoted MakeMKV source arg, got %q", arg)
	}
}

func TestJobRunnerCommandPreviewUsesTemporaryOutputPath(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Disc", "/tmp/intermediate.mkv", "00801.MPLS", "/remux/Disc.mkv"),
	}

	preview, err := runner.CommandPreview(req)
	if err != nil {
		t.Fatalf("CommandPreview returned error: %v", err)
	}
	if !strings.Contains(preview, "/remux/Disc.mkv.tmp") {
		t.Fatalf("expected preview to use temporary output path, got %q", preview)
	}
	if strings.Contains(preview, "\n  --output /remux/Disc.mkv\n") {
		t.Fatalf("expected preview not to use final output path directly, got %q", preview)
	}
}

func TestJobRunnerCommandPreviewRemapsSyntheticTrackIDsForIntermediateMKV(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	runner.inspectIntermediateTrackJSON = func(_ context.Context, path string) ([]byte, error) {
		if path != "/tmp/intermediate.mkv" {
			t.Fatalf("expected intermediate source path, got %q", path)
		}
		return []byte(`{
			"tracks":[
				{"id":0,"type":"video","properties":{"number":1}},
				{"id":5,"type":"audio","properties":{"number":2}},
				{"id":3,"type":"audio","properties":{"number":1}},
				{"id":7,"type":"subtitles","properties":{"number":1}}
			]
		}`), nil
	}
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/tmp/intermediate.mkv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":0}],
				"subtitles":[{"id":"subtitle-0","name":"English PGS","language":"eng","selected":true,"sourceIndex":0}]
			},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	preview, err := runner.CommandPreview(req)
	if err != nil {
		t.Fatalf("CommandPreview returned error: %v", err)
	}
	if !strings.Contains(preview, "--audio-tracks 3") {
		t.Fatalf("expected preview to use remapped audio ID, got %q", preview)
	}
	if !strings.Contains(preview, "--subtitle-tracks 7") {
		t.Fatalf("expected preview to use remapped subtitle ID, got %q", preview)
	}
	if strings.Contains(preview, "--audio-tracks audio-0") {
		t.Fatalf("expected preview not to use synthetic audio ID, got %q", preview)
	}
}

func TestJobRunnerExecuteRenamesTemporaryOutputAfterSuccessfulRun(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	tempPath := finalPath + ".tmp"

	runner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			if draft.OutputPath != tempPath {
				t.Fatalf("expected runner output path %q, got %q", tempPath, draft.OutputPath)
			}
			if err := os.WriteFile(draft.OutputPath, []byte("muxed"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if onOutput != nil {
				onOutput("Progress: 100%")
			}
			return "Progress: 100%", nil
		},
	})

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Disc", "/tmp/intermediate.mkv", "00801.MPLS", finalPath),
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected final output to exist: %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
}

func TestJobRunnerExecuteRemovesTemporaryOutputAfterFailure(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	tempPath := finalPath + ".tmp"

	runner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			if err := os.WriteFile(draft.OutputPath, []byte("partial"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			return "", errors.New("runner exploded")
		},
	})

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Disc", "/tmp/intermediate.mkv", "00801.MPLS", finalPath),
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "runner exploded") {
		t.Fatalf("expected runner error, got %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
	if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected final output not to exist, got %v", err)
	}
}

func TestJobRunnerExecuteRemovesStaleTemporaryOutputBeforeRun(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	tempPath := finalPath + ".tmp"

	if err := os.WriteFile(tempPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	runner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			content, err := os.ReadFile(draft.OutputPath)
			if err == nil && string(content) == "stale" {
				t.Fatalf("expected stale temporary output to be removed before run")
			}
			if err := os.WriteFile(draft.OutputPath, []byte("fresh"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			return "", nil
		},
	})

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Disc", "/tmp/intermediate.mkv", "00801.MPLS", finalPath),
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "fresh" {
		t.Fatalf("expected final output to contain fresh data, got %q", string(content))
	}
}

func TestBuildExecutionDraftPreservesSourceIndexFromPayload(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/bd_input/Disc","type":"bdmv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":4}],
				"subtitles":[{"id":"subtitle-0","name":"English PGS","language":"eng","selected":true,"sourceIndex":2}]
			},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	draft, err := runner.BuildExecutionDraft(req)
	if err != nil {
		t.Fatalf("BuildExecutionDraft returned error: %v", err)
	}
	if len(draft.Audio) != 1 || draft.Audio[0].SourceIndex != 4 {
		t.Fatalf("expected audio sourceIndex 4, got %+v", draft.Audio)
	}
	if len(draft.Subtitles) != 1 || draft.Subtitles[0].SourceIndex != 2 {
		t.Fatalf("expected subtitle sourceIndex 2, got %+v", draft.Subtitles)
	}
}

func TestRemapDraftTracksBySourceIndexUsesIntermediateMKVTrackIDs(t *testing.T) {
	draft := Draft{
		Audio: []AudioTrack{
			{ID: "audio-0", Name: "English", Language: "eng", Selected: true, SourceIndex: 0},
			{ID: "audio-1", Name: "Japanese", Language: "jpn", Selected: true, SourceIndex: 1},
		},
		Subtitles: []SubtitleTrack{
			{ID: "subtitle-0", Name: "English PGS", Language: "eng", Selected: true, SourceIndex: 0},
		},
	}
	jsonOutput := `{
		"tracks":[
			{"id":0,"type":"video","properties":{"number":1}},
			{"id":3,"type":"audio","properties":{"number":1}},
			{"id":5,"type":"audio","properties":{"number":2}},
			{"id":7,"type":"subtitles","properties":{"number":1}}
		]
	}`

	remapped, err := RemapDraftTrackIDsBySourceIndex(draft, []byte(jsonOutput))
	if err != nil {
		t.Fatalf("RemapDraftTrackIDsBySourceIndex returned error: %v", err)
	}
	if remapped.Audio[0].ID != "3" || remapped.Audio[1].ID != "5" {
		t.Fatalf("expected remapped audio IDs [3 5], got %+v", remapped.Audio)
	}
	if remapped.Subtitles[0].ID != "7" {
		t.Fatalf("expected remapped subtitle ID 7, got %+v", remapped.Subtitles)
	}
}

func TestRemapDraftTracksBySourceIndexOrdersSameTypeTracksByTrackNumber(t *testing.T) {
	draft := Draft{
		Audio: []AudioTrack{
			{ID: "audio-0", Name: "English", Language: "eng", Selected: true, SourceIndex: 0},
			{ID: "audio-1", Name: "Japanese", Language: "jpn", Selected: true, SourceIndex: 1},
		},
		Subtitles: []SubtitleTrack{
			{ID: "subtitle-0", Name: "English PGS", Language: "eng", Selected: true, SourceIndex: 0},
			{ID: "subtitle-1", Name: "Signs", Language: "eng", Selected: true, SourceIndex: 1},
		},
	}
	jsonOutput := `{
		"tracks":[
			{"id":11,"type":"audio","properties":{"number":3}},
			{"id":8,"type":"subtitles","properties":{"number":2}},
			{"id":0,"type":"video","properties":{"number":1}},
			{"id":9,"type":"subtitles","properties":{"number":1}},
			{"id":7,"type":"audio","properties":{"number":2}}
		]
	}`

	remapped, err := RemapDraftTrackIDsBySourceIndex(draft, []byte(jsonOutput))
	if err != nil {
		t.Fatalf("RemapDraftTrackIDsBySourceIndex returned error: %v", err)
	}
	if remapped.Audio[0].ID != "7" || remapped.Audio[1].ID != "11" {
		t.Fatalf("expected audio IDs ordered by track number [7 11], got %+v", remapped.Audio)
	}
	if remapped.Subtitles[0].ID != "9" || remapped.Subtitles[1].ID != "8" {
		t.Fatalf("expected subtitle IDs ordered by track number [9 8], got %+v", remapped.Subtitles)
	}
}

func TestRemapDraftTracksBySourceIndexFallsBackWhenTrackNumberMissing(t *testing.T) {
	draft := Draft{
		Audio: []AudioTrack{
			{ID: "audio-0", Name: "English", Language: "eng", Selected: true, SourceIndex: 0},
			{ID: "audio-1", Name: "Japanese", Language: "jpn", Selected: true, SourceIndex: 1},
		},
	}
	jsonOutput := `{
		"tracks":[
			{"id":5,"type":"audio","properties":{"number":0}},
			{"id":3,"type":"audio","properties":{"number":2}},
			{"id":0,"type":"video","properties":{"number":1}}
		]
	}`

	remapped, err := RemapDraftTrackIDsBySourceIndex(draft, []byte(jsonOutput))
	if err != nil {
		t.Fatalf("RemapDraftTrackIDsBySourceIndex returned error: %v", err)
	}
	if remapped.Audio[0].ID != "3" || remapped.Audio[1].ID != "5" {
		t.Fatalf("expected numbered audio first and fallback audio second, got %+v", remapped.Audio)
	}
}

func TestLookupMakeMKVTitlePlaylistParsesRobotOutput(t *testing.T) {
	robotOutput := strings.Join([]string{
		`TINFO:0,2,0,"Main Feature"`,
		`TINFO:0,16,0,"00800"`,
		`TINFO:1,2,0,"Bonus"`,
		`TINFO:1,16,0,"00001"`,
	}, "\n")

	titleID, err := LookupMakeMKVTitleIDByPlaylist([]byte(robotOutput), "00800.MPLS")
	if err != nil {
		t.Fatalf("LookupMakeMKVTitleIDByPlaylist returned error: %v", err)
	}
	if titleID != 0 {
		t.Fatalf("expected title id 0, got %d", titleID)
	}
}

func TestLookupMakeMKVTitleIDByPlaylistReturnsFirstDuplicateMatch(t *testing.T) {
	robotOutput := strings.Join([]string{
		`TINFO:7,16,0,"00800"`,
		`TINFO:3,16,0,"00001"`,
		`TINFO:2,16,0,"00800"`,
	}, "\n")

	titleID, err := LookupMakeMKVTitleIDByPlaylist([]byte(robotOutput), "00800.MPLS")
	if err != nil {
		t.Fatalf("LookupMakeMKVTitleIDByPlaylist returned error: %v", err)
	}
	if titleID != 7 {
		t.Fatalf("expected first matching title id 7, got %d", titleID)
	}
}

func TestJobRunnerExecuteRunsMakeMKVTwoStageFlowForBDMVSource(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	intermediateDir := filepath.Join(outputRoot, "makemkv")
	intermediatePath := filepath.Join(intermediateDir, "title_t00.mkv")
	calls := make([]string, 0, 4)
	capturingRunner := fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			calls = append(calls, "mkvmerge")
			if draft.SourcePath != intermediatePath {
				t.Fatalf("expected second pass to use intermediate mkv %q, got %q", intermediatePath, draft.SourcePath)
			}
			if len(draft.Audio) != 1 || draft.Audio[0].ID != "3" {
				t.Fatalf("expected remapped audio track id 3, got %+v", draft.Audio)
			}
			if len(draft.Subtitles) != 1 || draft.Subtitles[0].ID != "7" {
				t.Fatalf("expected remapped subtitle track id 7, got %+v", draft.Subtitles)
			}
			if err := os.WriteFile(draft.OutputPath, []byte("muxed"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if onOutput != nil {
				onOutput("second pass")
			}
			return "second pass", nil
		},
	}
	jobRunner := NewJobRunner(capturingRunner)
	jobRunner.tempDir = func() string {
		return intermediateDir
	}
	jobRunner.runMakeMKVInfo = func(_ context.Context, sourcePath string) ([]byte, error) {
		calls = append(calls, "info")
		if sourcePath != "/bd_input/Disc" {
			t.Fatalf("expected MakeMKV info source /bd_input/Disc, got %q", sourcePath)
		}
		return []byte(strings.Join([]string{
			`TINFO:4,16,0,"00801"`,
			`TINFO:5,16,0,"00001"`,
		}, "\n")), nil
	}
	jobRunner.runMakeMKVMKV = func(_ context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error {
		calls = append(calls, "makemkv")
		if sourcePath != "/bd_input/Disc" {
			t.Fatalf("expected MakeMKV mkv source /bd_input/Disc, got %q", sourcePath)
		}
		if titleID != 4 {
			t.Fatalf("expected MakeMKV title id 4, got %d", titleID)
		}
		if tempDir != intermediateDir {
			t.Fatalf("expected MakeMKV temp dir %q, got %q", intermediateDir, tempDir)
		}
		if err := os.WriteFile(intermediatePath, []byte("intermediate"), 0o644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if onOutput != nil {
			onOutput("makemkv")
		}
		return nil
	}
	jobRunner.inspectIntermediateTrackJSON = func(_ context.Context, path string) ([]byte, error) {
		calls = append(calls, "identify")
		if path != intermediatePath {
			t.Fatalf("expected identify path %q, got %q", intermediatePath, path)
		}
		return []byte(`{
			"tracks":[
				{"id":0,"type":"video","properties":{"number":1}},
				{"id":3,"type":"audio","properties":{"number":1}},
				{"id":7,"type":"subtitles","properties":{"number":1}}
			]
		}`), nil
	}

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/bd_input/Disc","type":"bdmv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":0}],
				"subtitles":[{"id":"subtitle-0","name":"English PGS","language":"eng","selected":true,"sourceIndex":0}]
			},
			"outputPath":"` + finalPath + `"
		}`,
	}

	_, _, err := jobRunner.Execute(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Join(calls, ",") != "info,makemkv,identify,mkvmerge" {
		t.Fatalf("expected call order info,makemkv,identify,mkvmerge, got %v", calls)
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected final output to exist: %v", err)
	}
}

func TestJobRunnerExecuteCleansTempDirWhenIntermediateIdentifyFails(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	intermediateDir := filepath.Join(outputRoot, "makemkv")
	intermediatePath := filepath.Join(intermediateDir, "title_t00.mkv")

	jobRunner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			t.Fatalf("second-pass mkvmerge should not run when identify fails")
			return "", nil
		},
	})
	jobRunner.tempDir = func() string {
		return intermediateDir
	}
	jobRunner.runMakeMKVInfo = func(_ context.Context, sourcePath string) ([]byte, error) {
		return []byte(`TINFO:4,16,0,"00801"`), nil
	}
	jobRunner.runMakeMKVMKV = func(_ context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error {
		if err := os.WriteFile(intermediatePath, []byte("intermediate"), 0o644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		return nil
	}
	jobRunner.inspectIntermediateTrackJSON = func(_ context.Context, path string) ([]byte, error) {
		return nil, errors.New("identify failed")
	}

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/bd_input/Disc","type":"bdmv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":0}],
				"subtitles":[]
			},
			"outputPath":"` + finalPath + `"
		}`,
	}

	_, _, err := jobRunner.Execute(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "identify failed") {
		t.Fatalf("expected identify failure, got %v", err)
	}
	if _, err := os.Stat(intermediateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected intermediate temp dir to be cleaned up, got %v", err)
	}
}

func TestJobRunnerExecuteCleansTempDirWhenIntermediateDiscoveryFails(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	intermediateDir := filepath.Join(outputRoot, "makemkv")

	jobRunner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			t.Fatalf("second-pass mkvmerge should not run when intermediate mkv discovery fails")
			return "", nil
		},
	})
	jobRunner.tempDir = func() string {
		return intermediateDir
	}
	jobRunner.runMakeMKVInfo = func(_ context.Context, sourcePath string) ([]byte, error) {
		return []byte(`TINFO:4,16,0,"00801"`), nil
	}
	jobRunner.runMakeMKVMKV = func(_ context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error {
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		return nil
	}
	jobRunner.locateIntermediateMKV = func(tempDir string) (string, error) {
		return "", errors.New("intermediate mkv not found")
	}

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/bd_input/Disc","type":"bdmv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":0}],
				"subtitles":[]
			},
			"outputPath":"` + finalPath + `"
		}`,
	}

	_, _, err := jobRunner.Execute(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "intermediate mkv not found") {
		t.Fatalf("expected intermediate discovery failure, got %v", err)
	}
	if _, err := os.Stat(intermediateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected intermediate temp dir to be cleaned up, got %v", err)
	}
}

func TestJobRunnerExecuteRemapsSyntheticTrackIDsForIntermediateMKV(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	capturingRunner := &stubRunner{}
	runner := NewJobRunner(capturingRunner)
	runner.inspectIntermediateTrackJSON = func(_ context.Context, path string) ([]byte, error) {
		if path != "/tmp/intermediate.mkv" {
			t.Fatalf("expected intermediate source path, got %q", path)
		}
		return []byte(`{
			"tracks":[
				{"id":0,"type":"video","properties":{"number":1}},
				{"id":3,"type":"audio","properties":{"number":1}},
				{"id":5,"type":"audio","properties":{"number":2}},
				{"id":7,"type":"subtitles","properties":{"number":1}}
			]
		}`), nil
	}

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON: `{
			"source":{"name":"Disc","path":"/tmp/intermediate.mkv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":1}],
				"subtitles":[{"id":"subtitle-0","name":"English PGS","language":"eng","selected":true,"sourceIndex":0}]
			},
			"outputPath":"` + finalPath + `"
		}`,
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if capturingRunner.lastDraft.Audio[0].ID != "5" {
		t.Fatalf("expected remapped audio ID 5, got %+v", capturingRunner.lastDraft.Audio)
	}
	if capturingRunner.lastDraft.Subtitles[0].ID != "7" {
		t.Fatalf("expected remapped subtitle ID 7, got %+v", capturingRunner.lastDraft.Subtitles)
	}
}

func TestJobRunnerExecuteRemovesTemporaryOutputWhenFinalizeRenameFails(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	tempPath := finalPath + ".tmp"

	runner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			if err := os.WriteFile(draft.OutputPath, []byte("muxed"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			return "", nil
		},
	})
	runner.renameOutput = func(_, _ string) error {
		return errors.New("rename failed")
	}

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Disc", "/tmp/intermediate.mkv", "00801.MPLS", finalPath),
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("expected rename failure, got %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
}
