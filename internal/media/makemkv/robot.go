package makemkv

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const forcedSubtitleFlag uint64 = 1 << 12

type RobotInfo struct {
	Titles []TitleInfo
}

type TitleInfo struct {
	TitleID      int
	PlaylistName string
	Name         string
	Tracks       []TrackInfo
}

type TrackInfo struct {
	StreamIndex   int
	Type          string
	LangCode      string
	LangName      string
	CodecID       string
	CodecShort    string
	CodecLong     string
	Channels      int
	ChannelLayout string
	StreamFlags   uint64
	MkvFlags      string
	MkvFlagsText  string
	DisplayName   string
}

type VisibleTrack struct {
	ID          string `json:"id"`
	SourceIndex int    `json:"sourceIndex"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	CodecLabel  string `json:"codecLabel,omitempty"`
	Selected    bool   `json:"selected"`
	Default     bool   `json:"default"`
	Forced      bool   `json:"forced,omitempty"`
}

type TitleView struct {
	PlaylistName string
	TitleID      int
	Audio        []VisibleTrack
	Subtitles    []VisibleTrack
}

func ParseRobotOutput(raw []byte) (RobotInfo, error) {
	byTitle := map[int]*TitleInfo{}
	byTrack := map[trackKey]*TrackInfo{}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "TINFO:"):
			if err := applyTitleInfo(strings.TrimPrefix(line, "TINFO:"), byTitle); err != nil {
				return RobotInfo{}, err
			}
		case strings.HasPrefix(line, "SINFO:"):
			if err := applyStreamInfo(strings.TrimPrefix(line, "SINFO:"), byTitle, byTrack); err != nil {
				return RobotInfo{}, err
			}
		}
	}

	if len(byTitle) == 0 {
		return RobotInfo{}, errors.New("makemkv robot output does not contain titles")
	}

	titleIDs := make([]int, 0, len(byTitle))
	for titleID := range byTitle {
		titleIDs = append(titleIDs, titleID)
	}
	sort.Ints(titleIDs)

	info := RobotInfo{Titles: make([]TitleInfo, 0, len(titleIDs))}
	for _, titleID := range titleIDs {
		title := *byTitle[titleID]
		sort.Slice(title.Tracks, func(i, j int) bool {
			return title.Tracks[i].StreamIndex < title.Tracks[j].StreamIndex
		})
		info.Titles = append(info.Titles, title)
	}
	return info, nil
}

func (r RobotInfo) TitleByPlaylist(playlistName string) (TitleInfo, error) {
	normalized := normalizePlaylist(playlistName)
	if normalized == "" {
		return TitleInfo{}, errors.New("playlist name is required")
	}
	for _, title := range r.Titles {
		if normalizePlaylist(title.PlaylistName) == normalized {
			return title, nil
		}
	}
	return TitleInfo{}, fmt.Errorf("playlist %s not found in makemkv robot output", normalized)
}

func BuildTitleView(title TitleInfo) (TitleView, error) {
	audioTracks := filterCompatibilityAudio(collectTracksByType(title.Tracks, "Audio"))
	subtitleTracks := collectTracksByType(title.Tracks, "Subtitles")
	if len(audioTracks) == 0 && len(subtitleTracks) == 0 {
		return TitleView{}, errors.New("title does not contain usable tracks")
	}

	audio, err := makeVisibleAudio(audioTracks)
	if err != nil {
		return TitleView{}, err
	}
	subtitles, err := makeVisibleSubtitles(subtitleTracks)
	if err != nil {
		return TitleView{}, err
	}

	view := TitleView{
		PlaylistName: normalizePlaylist(title.PlaylistName),
		TitleID:      title.TitleID,
		Audio:        audio,
		Subtitles:    subtitles,
	}
	return view, nil
}

type trackKey struct {
	titleID     int
	streamIndex int
}

func applyTitleInfo(payload string, byTitle map[int]*TitleInfo) error {
	parts := strings.SplitN(payload, ",", 4)
	if len(parts) != 4 {
		return fmt.Errorf("invalid TINFO line: %s", payload)
	}

	titleID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid TINFO title id: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid TINFO code: %w", err)
	}
	value := decodeRobotValue(parts[3])

	title := ensureTitle(byTitle, titleID)
	switch code {
	case 2:
		title.Name = value
	case 16:
		title.PlaylistName = normalizePlaylist(value)
	}
	return nil
}

func applyStreamInfo(payload string, byTitle map[int]*TitleInfo, byTrack map[trackKey]*TrackInfo) error {
	parts := strings.SplitN(payload, ",", 5)
	if len(parts) != 5 {
		return fmt.Errorf("invalid SINFO line: %s", payload)
	}

	titleID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid SINFO title id: %w", err)
	}
	streamIndex, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid SINFO stream index: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return fmt.Errorf("invalid SINFO code: %w", err)
	}
	value := decodeRobotValue(parts[4])

	title := ensureTitle(byTitle, titleID)
	key := trackKey{titleID: titleID, streamIndex: streamIndex}
	track, ok := byTrack[key]
	if !ok {
		title.Tracks = append(title.Tracks, TrackInfo{StreamIndex: streamIndex})
		track = &title.Tracks[len(title.Tracks)-1]
		byTrack[key] = track
	}

	switch code {
	case 1:
		track.Type = value
	case 3:
		track.LangCode = value
	case 4:
		track.LangName = value
	case 5:
		track.CodecID = value
	case 6:
		track.CodecShort = value
	case 7:
		track.CodecLong = value
	case 14:
		track.Channels = parseInt(value)
	case 22:
		track.StreamFlags = parseUint(value)
	case 30:
		track.DisplayName = value
	case 38:
		track.MkvFlags = value
	case 39:
		track.MkvFlagsText = value
	case 40:
		track.ChannelLayout = value
	}
	return nil
}

func ensureTitle(byTitle map[int]*TitleInfo, titleID int) *TitleInfo {
	title, ok := byTitle[titleID]
	if ok {
		return title
	}
	title = &TitleInfo{TitleID: titleID}
	byTitle[titleID] = title
	return title
}

func decodeRobotValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func normalizePlaylist(value string) string {
	trimmed := decodeRobotValue(value)
	if trimmed == "" {
		return ""
	}
	if filepath.Ext(trimmed) == "" {
		trimmed += ".MPLS"
	}
	return strings.ToUpper(trimmed)
}

func collectTracksByType(tracks []TrackInfo, trackType string) []TrackInfo {
	filtered := make([]TrackInfo, 0, len(tracks))
	for _, track := range tracks {
		if strings.EqualFold(track.Type, trackType) {
			filtered = append(filtered, track)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StreamIndex < filtered[j].StreamIndex
	})
	return filtered
}

func filterCompatibilityAudio(tracks []TrackInfo) []TrackInfo {
	visible := make([]TrackInfo, 0, len(tracks))
	for _, track := range tracks {
		if len(visible) > 0 && isCompatibilityAudioTrack(visible[len(visible)-1], track) {
			continue
		}
		visible = append(visible, track)
	}
	return visible
}

func isCompatibilityAudioTrack(previous, current TrackInfo) bool {
	if current.StreamIndex != previous.StreamIndex+1 {
		return false
	}
	if !sameTrackLanguage(previous, current) {
		return false
	}
	if previous.Channels >= 6 && current.Channels <= 2 {
		return false
	}

	isDolbyCore := (previous.CodecID == "A_TRUEHD" || previous.CodecID == "A_EAC3") && current.CodecID == "A_AC3"
	if isDolbyCore {
		return true
	}

	isDTSCore := previous.CodecID == "A_DTS" && current.CodecID == "A_DTS" && !strings.EqualFold(strings.TrimSpace(previous.CodecShort), "DTS") && strings.EqualFold(strings.TrimSpace(current.CodecShort), "DTS")
	if isDTSCore {
		return true
	}

	return false
}

func sameTrackLanguage(previous, current TrackInfo) bool {
	previousLang := strings.ToLower(strings.TrimSpace(previous.LangCode))
	currentLang := strings.ToLower(strings.TrimSpace(current.LangCode))
	if previousLang != "" || currentLang != "" {
		return previousLang == currentLang
	}
	return strings.EqualFold(strings.TrimSpace(previous.LangName), strings.TrimSpace(current.LangName))
}

func makeVisibleAudio(tracks []TrackInfo) ([]VisibleTrack, error) {
	visible := make([]VisibleTrack, 0, len(tracks))
	for i, track := range tracks {
		name := displayName(track)
		language := trackLanguage(track)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(language) == "" {
			return nil, fmt.Errorf("audio track %d missing required visible fields", track.StreamIndex)
		}
		visible = append(visible, VisibleTrack{
			ID:          fmt.Sprintf("A%d", i+1),
			SourceIndex: i,
			Name:        name,
			Language:    language,
			CodecLabel:  audioCodecLabel(track),
			Selected:    true,
			Default:     isDefaultTrack(track),
		})
	}
	return visible, nil
}

func makeVisibleSubtitles(tracks []TrackInfo) ([]VisibleTrack, error) {
	visible := make([]VisibleTrack, 0, len(tracks))
	for i, track := range tracks {
		name := subtitleDisplayName(track)
		language := trackLanguage(track)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(language) == "" {
			return nil, fmt.Errorf("subtitle track %d missing required visible fields", track.StreamIndex)
		}
		visible = append(visible, VisibleTrack{
			ID:          fmt.Sprintf("S%d", i+1),
			SourceIndex: i,
			Name:        name,
			Language:    language,
			Selected:    true,
			Default:     isDefaultTrack(track),
			Forced:      isForcedSubtitle(track),
		})
	}
	return visible, nil
}

func displayName(track TrackInfo) string {
	if strings.TrimSpace(track.DisplayName) != "" {
		return strings.TrimSpace(track.DisplayName)
	}
	if strings.EqualFold(strings.TrimSpace(track.Type), "Audio") {
		if derived := derivedAudioDisplayName(track); derived != "" {
			return derived
		}
	}
	if strings.TrimSpace(track.LangName) != "" {
		return strings.TrimSpace(track.LangName)
	}
	return strings.TrimSpace(track.LangCode)
}

func derivedAudioDisplayName(track TrackInfo) string {
	parts := make([]string, 0, 3)
	codec := strings.TrimSpace(track.CodecShort)
	if codec == "" {
		codec = strings.TrimSpace(track.CodecLong)
	}
	if codec != "" {
		parts = append(parts, codec)
	}
	if layout := strings.TrimSpace(track.ChannelLayout); layout != "" {
		parts = append(parts, layout)
	}
	if lang := strings.TrimSpace(track.LangName); lang != "" {
		parts = append(parts, lang)
	}
	return strings.Join(parts, " ")
}

func subtitleDisplayName(track TrackInfo) string {
	name := strings.TrimSpace(track.LangName)
	if name == "" {
		name = displayName(track)
	}
	if isForcedSubtitle(track) && !strings.Contains(strings.ToLower(name), "(forced only)") {
		if name == "" {
			return "forced only"
		}
		return name + " (forced only)"
	}
	return name
}

func trackLanguage(track TrackInfo) string {
	if strings.TrimSpace(track.LangCode) != "" {
		return strings.TrimSpace(track.LangCode)
	}
	return strings.TrimSpace(track.LangName)
}

func audioCodecLabel(track TrackInfo) string {
	base := strings.TrimSpace(track.CodecShort)
	layout := strings.TrimSpace(track.ChannelLayout)
	switch {
	case base != "" && layout != "":
		return base + "." + layout
	case base != "":
		return base
	default:
		return strings.TrimSpace(track.CodecID)
	}
}

func isForcedSubtitle(track TrackInfo) bool {
	if track.StreamFlags&forcedSubtitleFlag != 0 {
		return true
	}
	return strings.Contains(strings.ToLower(track.DisplayName), "(forced only)")
}

func isDefaultTrack(track TrackInfo) bool {
	return hasDefaultMarker(track.MkvFlags) || hasDefaultMarker(track.MkvFlagsText)
}

func hasDefaultMarker(value string) bool {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return false
	}
	for _, part := range strings.FieldsFunc(normalized, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '/'
	}) {
		if strings.TrimSpace(part) == "default" {
			return true
		}
	}
	return false
}

func parseInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

func parseUint(value string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 0, 64)
	if err != nil {
		return 0
	}
	return n
}
