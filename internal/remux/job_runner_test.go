package remux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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

func TestBuildExecutionDraftPreservesMakeMKVCacheAndSourceIndexes(t *testing.T) {
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
				"title":"Disc",
				"playlistName":"00801.MPLS",
				"dvMergeEnabled":true,
				"segmentPaths":["/bd_input/Disc/BDMV/STREAM/00001.m2ts"],
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p","hdrType":"DV.HDR"},
				"audio":[{"id":"A1","sourceIndex":0,"name":"English Atmos","language":"eng","codecLabel":"TrueHD.7.1","default":true,"selected":true}],
				"subtitles":[{"id":"S1","sourceIndex":1,"name":"English","language":"eng","default":false,"selected":true,"forced":true}],
				"makemkv":{
					"playlistName":"00801.MPLS",
					"titleId":3,
					"audio":[{"id":"A1","sourceIndex":0,"name":"English Atmos","language":"eng","codecLabel":"TrueHD.7.1","default":true,"selected":true}],
					"subtitles":[{"id":"S1","sourceIndex":1,"name":"English","language":"eng","default":false,"selected":true,"forced":true}]
				}
			},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	draft, err := runner.BuildExecutionDraft(req)
	if err != nil {
		t.Fatalf("BuildExecutionDraft returned error: %v", err)
	}
	if draft.MakeMKV.TitleID != 3 || draft.MakeMKV.PlaylistName != "00801.MPLS" {
		t.Fatalf("expected MakeMKV cache metadata to be preserved, got %+v", draft.MakeMKV)
	}
	if len(draft.Audio) != 1 || draft.Audio[0].SourceIndex != 0 {
		t.Fatalf("expected audio sourceIndex to be preserved, got %+v", draft.Audio)
	}
	if len(draft.Subtitles) != 1 || draft.Subtitles[0].SourceIndex != 1 {
		t.Fatalf("expected subtitle sourceIndex to be preserved, got %+v", draft.Subtitles)
	}
	if len(draft.MakeMKV.Audio) != 1 || draft.MakeMKV.Audio[0].SourceIndex != 0 {
		t.Fatalf("expected MakeMKV audio cache to preserve sourceIndex, got %+v", draft.MakeMKV.Audio)
	}
	if len(draft.MakeMKV.Subtitles) != 1 || draft.MakeMKV.Subtitles[0].SourceIndex != 1 {
		t.Fatalf("expected MakeMKV subtitle cache to preserve sourceIndex, got %+v", draft.MakeMKV.Subtitles)
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

func TestJobRunnerCommandPreviewUsesTemporaryOutputPath(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validPayloadJSON("Disc", "/bd_input/Disc", "00801.MPLS", "/remux/Disc.mkv"),
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

func TestMakeMKVSourceArgumentUsesFilePrefixWithoutShellQuoting(t *testing.T) {
	arg := makeMKVSourceArg("/bd input/Disc")
	if arg != "file:/bd input/Disc" {
		t.Fatalf("expected unquoted MakeMKV source arg, got %q", arg)
	}
}

func TestMakeMKVSourceArgumentUsesDiscRootForBDMVDirectory(t *testing.T) {
	arg := makeMKVSourceArg("/bd_input/Disc/BDMV")
	if arg != "file:/bd_input/Disc" {
		t.Fatalf("expected BDMV directory to map to disc root, got %q", arg)
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
		if sourcePath != "/bd_input/Disc/BDMV" {
			t.Fatalf("expected raw MakeMKV info source /bd_input/Disc/BDMV, got %q", sourcePath)
		}
		return []byte(strings.Join([]string{
			`TINFO:4,16,0,"00801"`,
			`TINFO:5,16,0,"00001"`,
		}, "\n")), nil
	}
	jobRunner.runMakeMKVMKV = func(_ context.Context, sourcePath string, titleID int, tempDir string, onOutput func(string)) error {
		calls = append(calls, "makemkv")
		if sourcePath != "/bd_input/Disc/BDMV" {
			t.Fatalf("expected raw MakeMKV mkv source /bd_input/Disc/BDMV, got %q", sourcePath)
		}
		if titleID != 4 {
			t.Fatalf("expected MakeMKV title id 4, got %d", titleID)
		}
		if tempDir != intermediateDir {
			t.Fatalf("expected MakeMKV temp dir %q, got %q", intermediateDir, tempDir)
		}
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
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
			"source":{"name":"Disc","path":"/bd_input/Disc/BDMV","type":"bdmv"},
			"bdinfo":{"playlistName":"00801.MPLS"},
			"draft":{
				"playlistName":"00801.MPLS",
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
				"audio":[{"id":"audio-0","name":"English","language":"eng","selected":true,"sourceIndex":0}],
				"subtitles":[{"id":"subtitle-0","name":"English PGS","language":"eng","selected":true,"sourceIndex":0}],
				"makemkv":{
					"playlistName":"00801.MPLS",
					"titleId":4,
					"audio":[{"id":"A1","sourceIndex":0,"name":"English","language":"eng","selected":true}],
					"subtitles":[{"id":"S1","sourceIndex":0,"name":"English PGS","language":"eng","selected":true}]
				}
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

func TestDefaultRunMakeMKVCommandsUseAbsoluteBinaryPath(t *testing.T) {
	jobRunner := NewJobRunner(&stubRunner{})
	root := t.TempDir()
	stubBinary := filepath.Join(root, "makemkvcon")
	logPath := filepath.Join(root, "invocations.log")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\n' \"$0 $*\" >> \"" + logPath + "\"",
		"if [ \"$1\" = \"info\" ]; then",
		"  printf 'TINFO:4,16,0,\"00801\"\\n'",
		"  exit 0",
		"fi",
		"if [ \"$3\" = \"mkv\" ]; then",
		"  outdir=$6",
		"  mkdir -p \"$outdir\"",
		"  : > \"$outdir/title_t00.mkv\"",
		"  exit 0",
		"fi",
		"exit 99",
	}, "\n")
	if err := os.WriteFile(stubBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	originalPath := makemkvconBinaryPath
	makemkvconBinaryPath = stubBinary
	defer func() {
		makemkvconBinaryPath = originalPath
	}()

	infoOutput, err := jobRunner.defaultRunMakeMKVInfo(context.Background(), "/bd_input/Disc/BDMV")
	if err != nil {
		t.Fatalf("defaultRunMakeMKVInfo returned error: %v", err)
	}
	if titleID, lookupErr := LookupMakeMKVTitleIDByPlaylist(infoOutput, "00801.MPLS"); lookupErr != nil || titleID != 4 {
		t.Fatalf("expected robot info for playlist lookup, got titleID=%d err=%v output=%q", titleID, lookupErr, string(infoOutput))
	}
	if err := jobRunner.defaultRunMakeMKVMKV(context.Background(), "/bd_input/Disc/BDMV", 4, t.TempDir(), nil); err != nil {
		t.Fatalf("defaultRunMakeMKVMKV returned error: %v", err)
	}
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 invocations, got %d: %q", len(lines), string(contents))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, stubBinary+" ") {
			t.Fatalf("expected absolute binary path %q in invocation, got %q", stubBinary, line)
		}
	}
	if !strings.Contains(lines[0], " info file:/bd_input/Disc --robot") {
		t.Fatalf("expected info invocation, got %q", lines[0])
	}
	if !strings.Contains(lines[1], " --messages=-null --progress=-stdout mkv file:/bd_input/Disc "+strconv.Itoa(4)+" ") {
		t.Fatalf("expected mkv invocation, got %q", lines[1])
	}
}

func TestBuildExecutionDraftRequiresMakeMKVCacheForBDMVSource(t *testing.T) {
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
				"title":"Disc",
				"playlistName":"00801.MPLS",
				"audio":[{"id":"A1","sourceIndex":0,"name":"English","language":"eng","selected":true}],
				"subtitles":[{"id":"S1","sourceIndex":1,"name":"English","language":"eng","selected":true}]
			},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	_, err := runner.BuildExecutionDraft(req)
	if err == nil || !strings.Contains(err.Error(), "makemkv cache") {
		t.Fatalf("expected missing makemkv cache error, got %v", err)
	}
}

func TestBuildExecutionDraftRejectsMismatchedMakeMKVPlaylist(t *testing.T) {
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
				"title":"Disc",
				"playlistName":"00801.MPLS",
				"audio":[{"id":"A1","sourceIndex":0,"name":"English","language":"eng","selected":true}],
				"makemkv":{
					"playlistName":"00001.MPLS",
					"titleId":3,
					"audio":[{"id":"A1","sourceIndex":0,"name":"English","language":"eng","selected":true}]
				}
			},
			"outputPath":"/remux/Disc.mkv"
		}`,
	}

	_, err := runner.BuildExecutionDraft(req)
	if err == nil || !strings.Contains(err.Error(), "makemkv cache playlist") {
		t.Fatalf("expected mismatched playlist error, got %v", err)
	}
}
