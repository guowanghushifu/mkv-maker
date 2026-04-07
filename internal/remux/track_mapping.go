package remux

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type makeMKVRobotField struct {
	titleID int
	code    int
	value   string
}

type mkvmergeTrackJSON struct {
	ID         int                      `json:"id"`
	Type       string                   `json:"type"`
	Properties mkvmergeTrackJSONDetails `json:"properties"`
}

type mkvmergeTrackJSONDetails struct {
	Number      int    `json:"number"`
	TagSourceID string `json:"tag_source_id"`
}

type mkvmergeIdentifyOutput struct {
	Tracks []mkvmergeTrackJSON `json:"tracks"`
}

func LookupMakeMKVTitleIDByPlaylist(robotOutput []byte, playlistName string) (int, error) {
	targetPlaylist := normalizePlaylistValue(playlistName)
	if targetPlaylist == "" {
		return 0, errors.New("playlist name is required")
	}

	for _, line := range strings.Split(string(robotOutput), "\n") {
		field, ok := parseMakeMKVRobotField(line)
		if !ok || field.code != 16 {
			continue
		}
		if normalizePlaylistValue(field.value) == targetPlaylist {
			return field.titleID, nil
		}
	}

	return 0, fmt.Errorf("playlist %s not found in makemkv robot output", targetPlaylist)
}

func parseMakeMKVRobotField(line string) (makeMKVRobotField, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "TINFO:") {
		return makeMKVRobotField{}, false
	}

	parts := strings.SplitN(strings.TrimPrefix(trimmed, "TINFO:"), ",", 4)
	if len(parts) != 4 {
		return makeMKVRobotField{}, false
	}

	titleID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return makeMKVRobotField{}, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return makeMKVRobotField{}, false
	}

	return makeMKVRobotField{
		titleID: titleID,
		code:    code,
		value:   strings.Trim(strings.TrimSpace(parts[3]), `"`),
	}, true
}

func normalizePlaylistValue(value string) string {
	trimmed := strings.TrimSpace(strings.Trim(value, `"`))
	if trimmed == "" {
		return ""
	}
	if filepath.Ext(trimmed) == "" {
		trimmed += ".MPLS"
	}
	return strings.ToUpper(trimmed)
}

type ResolvedTrackSelector struct {
	SourceIndex int
	TrackID     string
}

func BuildResolvedTrackSelectorsBySourceIndex(draft Draft, identifyJSON []byte) ([]ResolvedTrackSelector, []ResolvedTrackSelector, error) {
	var output mkvmergeIdentifyOutput
	if err := json.Unmarshal(identifyJSON, &output); err != nil {
		return nil, nil, err
	}

	audioTrackIDs := collectTrackIDsByType(output.Tracks, "audio")
	subtitleTrackIDs := collectTrackIDsByType(output.Tracks, "subtitles")

	audioSelectors := make([]ResolvedTrackSelector, 0, len(draft.Audio))
	for _, track := range draft.Audio {
		if !track.Selected {
			continue
		}
		mappedID, err := trackIDForSourceIndex(audioTrackIDs, track.SourceIndex, "audio")
		if err != nil {
			return nil, nil, err
		}
		audioSelectors = append(audioSelectors, ResolvedTrackSelector{SourceIndex: track.SourceIndex, TrackID: mappedID})
	}

	subtitleSelectors := make([]ResolvedTrackSelector, 0, len(draft.Subtitles))
	for _, track := range draft.Subtitles {
		if !track.Selected {
			continue
		}
		mappedID, err := trackIDForSourceIndex(subtitleTrackIDs, track.SourceIndex, "subtitle")
		if err != nil {
			return nil, nil, err
		}
		subtitleSelectors = append(subtitleSelectors, ResolvedTrackSelector{SourceIndex: track.SourceIndex, TrackID: mappedID})
	}

	return audioSelectors, subtitleSelectors, nil
}

func RemapDraftTrackIDsBySourceIndex(draft Draft, identifyJSON []byte) (Draft, error) {
	audioSelectors, subtitleSelectors, err := BuildResolvedTrackSelectorsBySourceIndex(draft, identifyJSON)
	if err != nil {
		return Draft{}, err
	}

	remapped := draft
	resolvedAudio := make(map[int]string, len(audioSelectors))
	for _, selector := range audioSelectors {
		resolvedAudio[selector.SourceIndex] = selector.TrackID
	}
	resolvedSubtitles := make(map[int]string, len(subtitleSelectors))
	for _, selector := range subtitleSelectors {
		resolvedSubtitles[selector.SourceIndex] = selector.TrackID
	}
	for i, track := range remapped.Audio {
		if !usesSyntheticTrackID(track.ID) {
			continue
		}
		if mappedID, ok := resolvedAudio[track.SourceIndex]; ok {
			remapped.Audio[i].ID = mappedID
		}
	}
	for i, track := range remapped.Subtitles {
		if !usesSyntheticTrackID(track.ID) {
			continue
		}
		if mappedID, ok := resolvedSubtitles[track.SourceIndex]; ok {
			remapped.Subtitles[i].ID = mappedID
		}
	}
	return remapped, nil
}

func collectTracksByType(tracks []mkvmergeTrackJSON, kind string) []mkvmergeTrackJSON {
	typedTracks := make([]mkvmergeTrackJSON, 0, len(tracks))
	for _, track := range tracks {
		if !strings.EqualFold(track.Type, kind) {
			continue
		}
		typedTracks = append(typedTracks, track)
	}
	return typedTracks
}

func collectTrackIDsByType(tracks []mkvmergeTrackJSON, kind string) []string {
	typedTracks := collectTracksByType(tracks, kind)
	if strings.EqualFold(kind, "subtitles") {
		typedTracks = filterAdjacentDuplicateSubtitleTracks(typedTracks)
	}

	slices.SortStableFunc(typedTracks, func(a, b mkvmergeTrackJSON) int {
		aOrdinal, aHasOrdinal := explicitTrackOrdinal(a)
		bOrdinal, bHasOrdinal := explicitTrackOrdinal(b)
		switch {
		case aHasOrdinal && bHasOrdinal && aOrdinal != bOrdinal:
			return aOrdinal - bOrdinal
		case aHasOrdinal != bHasOrdinal:
			if aHasOrdinal {
				return -1
			}
			return 1
		case a.ID != b.ID:
			return a.ID - b.ID
		default:
			return 0
		}
	})

	ids := make([]string, 0, len(typedTracks))
	for _, track := range typedTracks {
		ids = append(ids, strconv.Itoa(track.ID))
	}
	return ids
}

func filterAdjacentDuplicateSubtitleTracks(tracks []mkvmergeTrackJSON) []mkvmergeTrackJSON {
	if len(tracks) < 2 {
		return tracks
	}

	byID := append([]mkvmergeTrackJSON(nil), tracks...)
	slices.SortStableFunc(byID, func(a, b mkvmergeTrackJSON) int {
		return a.ID - b.ID
	})

	duplicateIDs := make(map[int]struct{})
	for i := 1; i < len(byID); i++ {
		previous := byID[i-1]
		current := byID[i]
		if current.ID != previous.ID+1 {
			continue
		}
		if previous.Properties.TagSourceID == "" || current.Properties.TagSourceID == "" {
			continue
		}
		if previous.Properties.TagSourceID != current.Properties.TagSourceID {
			continue
		}
		duplicateIDs[current.ID] = struct{}{}
	}

	filtered := make([]mkvmergeTrackJSON, 0, len(tracks))
	for _, track := range tracks {
		if _, duplicate := duplicateIDs[track.ID]; duplicate {
			continue
		}
		filtered = append(filtered, track)
	}
	return filtered
}

func explicitTrackOrdinal(track mkvmergeTrackJSON) (int, bool) {
	if track.Properties.Number > 0 {
		return track.Properties.Number, true
	}
	return 0, false
}

func usesSyntheticTrackID(id string) bool {
	trimmed := strings.TrimSpace(id)
	return strings.HasPrefix(trimmed, "audio-") || strings.HasPrefix(trimmed, "subtitle-")
}

func trackIDForSourceIndex(trackIDs []string, sourceIndex int, kind string) (string, error) {
	if sourceIndex < 0 || sourceIndex >= len(trackIDs) {
		return "", fmt.Errorf("%s sourceIndex %d is out of range", kind, sourceIndex)
	}
	return trackIDs[sourceIndex], nil
}
