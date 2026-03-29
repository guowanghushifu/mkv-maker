package remux

import (
	"context"
	"errors"
	"testing"
)

type stubRunner struct {
	lastDraft Draft
	output    string
	err       error
}

func (r *stubRunner) Run(_ context.Context, draft Draft) (string, error) {
	r.lastDraft = draft
	return r.output, r.err
}

func TestManagerStartRejectsWhenJobAlreadyRunning(t *testing.T) {
	manager := NewManager(&stubRunner{})
	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}

	_, err = manager.Start(StartRequest{
		SourceName:   "Second Disc",
		OutputName:   "Second.mkv",
		OutputPath:   "/remux/Second.mkv",
		PlaylistName: "00002.MPLS",
		PayloadJSON:  `{"source":{"name":"Second Disc"}}`,
	})
	if !errors.Is(err, ErrTaskAlreadyRunning) {
		t.Fatalf("expected ErrTaskAlreadyRunning, got %v", err)
	}
}

func TestManagerCurrentReturnsRunningAndLatestLog(t *testing.T) {
	manager := NewManager(&stubRunner{output: "mkvmerge progress"})

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Nightcrawler.mkv"
		}`,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if task.Status != "running" {
		t.Fatalf("expected running status, got %q", task.Status)
	}

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if current.ID != task.ID {
		t.Fatalf("expected current id %q, got %q", task.ID, current.ID)
	}
}
