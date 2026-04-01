package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/guowanghushifu/mkv-maker/internal/isomount"
	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type JobsHandler struct {
	Tasks      tasksManager
	InputDir   string
	OutputDir  string
	ISOManager ISOJobManager
}

type tasksManager interface {
	Start(req remux.StartRequest) (remux.Task, error)
	Current() (remux.Task, error)
	CurrentLog() (string, error)
}

type ISOJobManager interface {
	EnsureMounted(ctx context.Context, sourceID, isoPath string) (string, error)
	MarkInUse(sourceID string)
	MarkIdle(sourceID string)
	ReleaseSource(ctx context.Context, sourceID string) error
}

type isoJobManagerAdapter struct {
	manager *isomount.Manager
}

func NewISOJobManagerAdapter(manager *isomount.Manager) ISOJobManager {
	if manager == nil {
		return nil
	}
	return isoJobManagerAdapter{manager: manager}
}

func (a isoJobManagerAdapter) EnsureMounted(ctx context.Context, sourceID, isoPath string) (string, error) {
	return a.manager.EnsureMounted(ctx, sourceID, isoPath)
}

func (a isoJobManagerAdapter) MarkInUse(sourceID string) {
	a.manager.MarkInUse(sourceID)
}

func (a isoJobManagerAdapter) MarkIdle(sourceID string) {
	a.manager.MarkIdle(sourceID)
}

func (a isoJobManagerAdapter) ReleaseSource(ctx context.Context, sourceID string) error {
	_, err := a.manager.ReleaseSource(ctx, sourceID)
	return err
}

type createJobRequest struct {
	Source struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"source"`
	BDInfo struct {
		PlaylistName string `json:"playlistName"`
		RawText      string `json:"rawText"`
	} `json:"bdinfo"`
	Draft struct {
		PlaylistName string `json:"playlistName"`
	} `json:"draft"`
	OutputFilename string `json:"outputFilename"`
	OutputPath     string `json:"outputPath"`
}

const createJobBodyLimit = 2 << 20

func NewJobsHandler(tasks tasksManager, inputDir, outputDir string, isoManager ...any) *JobsHandler {
	var manager ISOJobManager
	if len(isoManager) > 0 {
		switch v := isoManager[0].(type) {
		case ISOJobManager:
			manager = v
		case *isomount.Manager:
			manager = NewISOJobManagerAdapter(v)
		}
	}
	return &JobsHandler{
		Tasks:      tasks,
		InputDir:   strings.TrimSpace(inputDir),
		OutputDir:  strings.TrimSpace(outputDir),
		ISOManager: manager,
	}
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, createJobBodyLimit)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var req createJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	sourceType := strings.ToLower(strings.TrimSpace(req.Source.Type))
	if sourceType != "" && sourceType != "bdmv" && sourceType != "iso" {
		http.Error(w, "only bdmv sources are supported", http.StatusBadRequest)
		return
	}
	if sourceType != "iso" && !isPathWithinRoot(h.InputDir, req.Source.Path) {
		http.Error(w, "source path is outside input root", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(req.Source.Path); err != nil {
		http.Error(w, "source path does not exist", http.StatusBadRequest)
		return
	}
	if err := validateNewOutputPathWithinRoot(h.OutputDir, req.OutputPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsed, err := mediabdinfo.Parse(req.BDInfo.RawText)
	if err != nil {
		http.Error(w, "invalid bdinfo payload", http.StatusBadRequest)
		return
	}

	playlistName := strings.ToUpper(strings.TrimSpace(parsed.PlaylistName))
	if playlistName == "" {
		http.Error(w, "missing playlist name", http.StatusBadRequest)
		return
	}
	playlistName = strings.ToUpper(filepath.Base(playlistName))
	if !playlistNamePattern.MatchString(playlistName) {
		http.Error(w, "invalid playlist name", http.StatusBadRequest)
		return
	}
	if requested := strings.ToUpper(strings.TrimSpace(req.BDInfo.PlaylistName)); requested != "" && requested != playlistName {
		http.Error(w, "bdinfo playlist mismatch", http.StatusBadRequest)
		return
	}
	if draftPlaylist := strings.ToUpper(strings.TrimSpace(req.Draft.PlaylistName)); draftPlaylist != "" && draftPlaylist != playlistName {
		http.Error(w, "draft playlist mismatch", http.StatusBadRequest)
		return
	}
	sourcePlaylistRoot := strings.TrimSpace(req.Source.Path)
	if strings.EqualFold(filepath.Base(sourcePlaylistRoot), "BDMV") {
		sourcePlaylistRoot = filepath.Dir(sourcePlaylistRoot)
	}
	payloadJSON := strings.TrimSpace(string(body))
	sourceID := strings.TrimSpace(req.Source.ID)
	isoMounted := false
	if sourceType == "iso" {
		if h.ISOManager == nil {
			http.Error(w, "iso manager is not configured", http.StatusInternalServerError)
			return
		}
		mountedRoot, err := h.ISOManager.EnsureMounted(r.Context(), sourceID, req.Source.Path)
		if err != nil {
			http.Error(w, "failed to mount iso source", http.StatusBadRequest)
			return
		}
		h.ISOManager.MarkInUse(sourceID)
		isoMounted = true
		sourcePlaylistRoot = mountedRoot
		payloadJSON, err = rewriteMountedISOPayloadJSON(body, mountedRoot)
		if err != nil {
			h.ISOManager.MarkIdle(sourceID)
			_ = h.ISOManager.ReleaseSource(context.Background(), sourceID)
			http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
			return
		}
	}
	if payloadJSON == "" {
		encoded, err := json.Marshal(req)
		if err != nil {
			http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
			return
		}
		payloadJSON = string(encoded)
	}
	if _, err := findPlaylistFilePath(sourcePlaylistRoot, playlistName); err != nil {
		if isoMounted {
			h.ISOManager.MarkIdle(sourceID)
			_ = h.ISOManager.ReleaseSource(context.Background(), sourceID)
		}
		http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
		return
	}
	startRequest := remux.StartRequest{
		SourceID:     sourceID,
		SourceType:   strings.TrimSpace(req.Source.Type),
		SourceName:   strings.TrimSpace(req.Source.Name),
		OutputName:   strings.TrimSpace(req.OutputFilename),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: playlistName,
		PayloadJSON:  payloadJSON,
	}
	task, err := h.Tasks.Start(startRequest)
	if err != nil {
		if isoMounted {
			h.ISOManager.MarkIdle(sourceID)
			_ = h.ISOManager.ReleaseSource(context.Background(), sourceID)
		}
		if errors.Is(err, remux.ErrTaskAlreadyRunning) {
			http.Error(w, "task already running", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func rewriteMountedISOPayloadJSON(body []byte, mountedRoot string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	source, _ := payload["source"].(map[string]any)
	if source == nil {
		source = map[string]any{}
		payload["source"] = source
	}
	source["path"] = mountedRoot
	source["type"] = "bdmv"

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (h *JobsHandler) Current(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	task, err := h.Tasks.Current()
	if err != nil {
		if errors.Is(err, remux.ErrTaskNotFound) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load current task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) CurrentLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	logText, err := h.Tasks.CurrentLog()
	if err != nil {
		if errors.Is(err, remux.ErrTaskNotFound) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load current task log", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(logText)); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}
