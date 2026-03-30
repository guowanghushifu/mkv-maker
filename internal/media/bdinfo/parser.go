package bdinfo

import (
	"bufio"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var ErrNoRecognizedFields = errors.New("no recognized bdinfo fields")
var ErrMissingPlaylist = errors.New("missing playlist name")

type Parsed struct {
	PlaylistName   string   `json:"playlistName"`
	DiscTitle      string   `json:"discTitle,omitempty"`
	Duration       string   `json:"duration,omitempty"`
	AudioLabels    []string `json:"audioLabels"`
	AudioCodecInfo []string `json:"-"`
	SubtitleLabels []string `json:"subtitleLabels"`
	RawText        string   `json:"rawText"`
	StreamFiles    []string `json:"-"`
	Video          Video    `json:"-"`
	DVMergeEnabled bool     `json:"-"`
}

type Video struct {
	Name       string `json:"name"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution"`
	HDRType    string `json:"hdrType,omitempty"`
}

type section int

const (
	sectionUnknown section = iota
	sectionPlaylistReport
	sectionVideo
	sectionAudio
	sectionSubtitles
	sectionFiles
)

type audioRow struct {
	Codec       string
	Language    string
	Description string
}

type subtitleRow struct {
	Language    string
	Description string
}

type videoRow struct {
	Codec       string
	Description string
	Hidden      bool
}

var (
	multiSpacePattern = regexp.MustCompile(`\s{2,}`)
	playlistPattern   = regexp.MustCompile(`(?i)\b(\d{5}\.MPLS)\b`)
	resolutionPattern = regexp.MustCompile(`(?i)\b(\d{3,4}p)\b`)
)

func Parse(rawText string) (Parsed, error) {
	parsed := Parsed{
		RawText:        rawText,
		AudioLabels:    []string{},
		AudioCodecInfo: []string{},
		SubtitleLabels: []string{},
		Video: Video{
			Name: "Main Video",
		},
	}

	foundField := false
	currentSection := sectionUnknown
	audioRows := make([]audioRow, 0, 8)
	subtitleRows := make([]subtitleRow, 0, 8)
	videoRows := make([]videoRow, 0, 2)
	scanner := bufio.NewScanner(strings.NewReader(rawText))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		upperLine := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upperLine, "DISC TITLE:"):
			parsed.DiscTitle = strings.TrimSpace(strings.TrimPrefix(line, "Disc Title:"))
			foundField = true
		case strings.HasPrefix(upperLine, "PLAYLIST REPORT"):
			currentSection = sectionPlaylistReport
			foundField = true
		case strings.HasPrefix(upperLine, "PLAYLIST:"):
			parsed.PlaylistName = strings.TrimSpace(strings.TrimPrefix(line, "PLAYLIST:"))
			foundField = true
		case strings.HasPrefix(upperLine, "VIDEO:"):
			inline := strings.TrimSpace(strings.TrimPrefix(line, "VIDEO:"))
			if inline != "" {
				videoRows = append(videoRows, videoRow{
					Codec:       "Video",
					Description: inline,
				})
				foundField = true
				continue
			}
			currentSection = sectionVideo
			foundField = true
		case strings.HasPrefix(upperLine, "AUDIO:"):
			inline := strings.TrimSpace(strings.TrimPrefix(line, "AUDIO:"))
			if inline != "" {
				audioRows = append(audioRows, parseInlineTrackRow(inline))
				foundField = true
				continue
			}
			currentSection = sectionAudio
			foundField = true
		case strings.HasPrefix(upperLine, "SUBTITLES:"):
			currentSection = sectionSubtitles
			foundField = true
		case strings.HasPrefix(upperLine, "FILES:"):
			currentSection = sectionFiles
			foundField = true
		case strings.HasPrefix(upperLine, "SUBTITLE:"):
			inline := strings.TrimSpace(strings.TrimPrefix(line, "SUBTITLE:"))
			if inline != "" {
				subtitleRows = append(subtitleRows, parseInlineSubtitleRow(inline))
				foundField = true
				continue
			}
			currentSection = sectionSubtitles
			foundField = true
		default:
			switch currentSection {
			case sectionPlaylistReport:
				if strings.HasPrefix(upperLine, "NAME:") {
					parsed.PlaylistName = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
					foundField = true
					continue
				}
				if strings.HasPrefix(upperLine, "LENGTH:") {
					parsed.Duration = strings.TrimSpace(strings.TrimPrefix(line, "Length:"))
					foundField = true
					continue
				}
			case sectionVideo:
				if row, ok := parseVideoTableRow(line); ok {
					videoRows = append(videoRows, row)
					foundField = true
				}
			case sectionAudio:
				if row, ok := parseAudioTableRow(line); ok {
					audioRows = append(audioRows, row)
					foundField = true
				}
			case sectionSubtitles:
				if row, ok := parseSubtitleTableRow(line); ok {
					subtitleRows = append(subtitleRows, row)
					foundField = true
				}
			case sectionFiles:
				if name, ok := parseFileTableRow(line); ok {
					parsed.StreamFiles = append(parsed.StreamFiles, name)
					foundField = true
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Parsed{}, err
	}
	if !foundField {
		return Parsed{}, ErrNoRecognizedFields
	}
	if parsed.PlaylistName == "" {
		if match := playlistPattern.FindStringSubmatch(rawText); len(match) == 2 {
			parsed.PlaylistName = strings.ToUpper(match[1])
		}
	}
	if parsed.PlaylistName == "" {
		return Parsed{}, ErrMissingPlaylist
	}

	for _, row := range audioRows {
		label := buildAudioLabel(row)
		if label != "" {
			parsed.AudioLabels = append(parsed.AudioLabels, label)
		}
		parsed.AudioCodecInfo = append(parsed.AudioCodecInfo, buildAudioCodecLabel(row))
	}
	for _, row := range subtitleRows {
		label := buildSubtitleLabel(row)
		if label != "" {
			parsed.SubtitleLabels = append(parsed.SubtitleLabels, label)
		}
	}
	if len(videoRows) > 0 {
		parsed.Video, parsed.DVMergeEnabled = buildVideo(videoRows, rawText)
	} else {
		parsed.Video.HDRType = inferHDRType(rawText, false)
		parsed.DVMergeEnabled = strings.Contains(strings.ToUpper(rawText), "DOLBY VISION")
	}

	return parsed, nil
}

func parseVideoTableRow(line string) (videoRow, bool) {
	if shouldIgnoreTableLine(line) {
		return videoRow{}, false
	}
	normalized := strings.TrimSpace(line)
	hidden := strings.HasPrefix(normalized, "*")
	if hidden {
		normalized = strings.TrimSpace(strings.TrimPrefix(normalized, "*"))
	}
	columns := splitColumns(normalized)
	if len(columns) == 0 {
		return videoRow{}, false
	}
	upperColumns := strings.ToUpper(strings.Join(columns, " "))
	if strings.Contains(upperColumns, "CODEC") && strings.Contains(upperColumns, "DESCRIPTION") {
		return videoRow{}, false
	}

	row := videoRow{
		Codec:  columns[0],
		Hidden: hidden,
	}
	if len(columns) > 1 {
		row.Description = columns[len(columns)-1]
	}
	return row, true
}

func parseAudioTableRow(line string) (audioRow, bool) {
	if shouldIgnoreTableLine(line) {
		return audioRow{}, false
	}
	columns := splitColumns(line)
	if len(columns) < 2 {
		return audioRow{}, false
	}
	if looksLikeAudioHeader(columns) {
		return audioRow{}, false
	}

	row := audioRow{
		Codec:    columns[0],
		Language: columns[1],
	}
	if len(columns) > 3 {
		row.Description = strings.Join(columns[3:], " ")
	} else if len(columns) > 2 {
		row.Description = columns[len(columns)-1]
	}
	return row, true
}

func parseSubtitleTableRow(line string) (subtitleRow, bool) {
	if shouldIgnoreTableLine(line) {
		return subtitleRow{}, false
	}
	columns := splitColumns(line)
	if len(columns) == 0 {
		return subtitleRow{}, false
	}
	if looksLikeSubtitleHeader(columns) {
		return subtitleRow{}, false
	}

	row := subtitleRow{
		Language: columns[0],
	}
	if len(columns) > 1 {
		row.Description = columns[len(columns)-1]
	}
	return row, true
}

func parseFileTableRow(line string) (string, bool) {
	if shouldIgnoreTableLine(line) {
		return "", false
	}
	columns := splitColumns(line)
	if len(columns) == 0 {
		return "", false
	}
	if strings.EqualFold(columns[0], "Name") {
		return "", false
	}
	if strings.HasSuffix(strings.ToUpper(columns[0]), ".M2TS") {
		return columns[0], true
	}
	return "", false
}

func splitColumns(line string) []string {
	parts := multiSpacePattern.Split(strings.TrimSpace(line), -1)
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			columns = append(columns, part)
		}
	}
	return columns
}

func shouldIgnoreTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if strings.Trim(trimmed, "- \t") == "" {
		return true
	}
	return false
}

func looksLikeAudioHeader(columns []string) bool {
	joined := strings.ToUpper(strings.Join(columns, " "))
	return strings.Contains(joined, "CODEC") && strings.Contains(joined, "LANGUAGE")
}

func looksLikeSubtitleHeader(columns []string) bool {
	joined := strings.ToUpper(strings.Join(columns, " "))
	return strings.Contains(joined, "LANGUAGE") && strings.Contains(joined, "DESCRIPTION")
}

func buildAudioLabel(row audioRow) string {
	suffix := extractDescriptiveSuffix(row.Description)
	if suffix != "" {
		return suffix
	}
	return strings.TrimSpace(strings.Join(compactLabelParts(row.Language, row.Codec), " "))
}

func buildAudioCodecLabel(row audioRow) string {
	combined := strings.ToUpper(strings.Join(compactLabelParts(row.Codec, row.Description), " / "))
	if combined == "" {
		return ""
	}

	parts := make([]string, 0, 3)
	switch {
	case strings.Contains(combined, "TRUEHD"):
		parts = append(parts, "TrueHD")
	case strings.Contains(combined, "DTS-HD MASTER"):
		parts = append(parts, "DTS-HD.MA")
	case strings.Contains(combined, "DTS-HD"):
		parts = append(parts, "DTS-HD")
	case strings.Contains(combined, "DOLBY DIGITAL PLUS"):
		parts = append(parts, "DDP")
	case strings.Contains(combined, "DOLBY DIGITAL"), strings.Contains(combined, "AC-3"):
		parts = append(parts, "DD")
	case strings.Contains(combined, "LPCM"):
		parts = append(parts, "LPCM")
	case strings.Contains(combined, "AAC"):
		parts = append(parts, "AAC")
	}

	if channels := extractChannelLayout(combined); channels != "" {
		parts = append(parts, channels)
	}
	switch {
	case strings.Contains(combined, "ATMOS"):
		parts = append(parts, "Atmos")
	case strings.Contains(combined, "DTS:X"):
		parts = append(parts, "DTS.X")
	}

	return strings.Join(parts, ".")
}

func extractChannelLayout(value string) string {
	for _, candidate := range []string{"7.1", "6.1", "5.1", "2.1", "2.0", "1.0"} {
		if strings.Contains(value, candidate) {
			return candidate
		}
	}
	return ""
}

func buildSubtitleLabel(row subtitleRow) string {
	suffix := extractDescriptiveSuffix(row.Description)
	if suffix != "" {
		return suffix
	}
	if row.Language != "" {
		return strings.TrimSpace(row.Language)
	}
	return strings.TrimSpace(row.Description)
}

func parseInlineTrackRow(body string) audioRow {
	parts := strings.Split(body, "/")
	row := audioRow{}
	if len(parts) > 0 {
		row.Language = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		row.Codec = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		row.Description = strings.TrimSpace(strings.Join(parts[2:], "/"))
	}
	return row
}

func parseInlineSubtitleRow(body string) subtitleRow {
	parts := strings.Split(body, "/")
	row := subtitleRow{}
	if len(parts) > 0 {
		row.Language = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		row.Description = strings.TrimSpace(strings.Join(parts[1:], "/"))
	}
	return row
}

func compactLabelParts(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func extractDescriptiveSuffix(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	segments := strings.Split(description, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment := strings.TrimSpace(segments[i])
		if segment == "" {
			continue
		}
		if isDescriptiveSegment(segment) {
			return trimDescriptiveSegment(segment)
		}
	}
	if isDescriptiveSegment(description) {
		return trimDescriptiveSegment(description)
	}
	return ""
}

func trimDescriptiveSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	for i, r := range segment {
		if unicode.In(r, unicode.Han) {
			return strings.TrimSpace(segment[i:])
		}
	}
	return segment
}

func isDescriptiveSegment(segment string) bool {
	for _, r := range segment {
		if unicode.In(r, unicode.Han) {
			return true
		}
	}
	lowered := strings.ToLower(segment)
	return strings.Contains(lowered, "commentary") ||
		strings.Contains(lowered, "forced") ||
		strings.Contains(lowered, "sdh")
}

func buildVideo(rows []videoRow, rawText string) (Video, bool) {
	video := Video{
		Name: "Main Video",
	}
	main := pickMainVideoRow(rows)
	video.Codec = normalizeVideoCodec(main.Codec)
	video.Resolution = extractResolution(main.Description)

	dvDetected := strings.Contains(strings.ToUpper(rawText), "DOLBY VISION")
	for _, row := range rows {
		upperRow := strings.ToUpper(row.Description + " " + row.Codec)
		if row.Hidden && strings.Contains(upperRow, "DOLBY VISION") {
			dvDetected = true
		}
		if strings.Contains(upperRow, "DOLBY VISION") {
			dvDetected = true
		}
		if video.Resolution == "" {
			video.Resolution = extractResolution(row.Description)
		}
	}
	video.HDRType = inferHDRType(rawText+" "+main.Description, dvDetected)
	return video, dvDetected
}

func pickMainVideoRow(rows []videoRow) videoRow {
	for _, row := range rows {
		if !row.Hidden {
			return row
		}
	}
	return rows[0]
}

func extractResolution(description string) string {
	match := resolutionPattern.FindStringSubmatch(description)
	if len(match) == 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

func inferHDRType(input string, dvDetected bool) string {
	upper := strings.ToUpper(input)
	if dvDetected || strings.Contains(upper, "HDR.DV") || strings.Contains(upper, "DOLBY VISION") {
		return "HDR.DV"
	}
	if strings.Contains(upper, "HDR") {
		return "HDR"
	}
	return ""
}

func normalizeVideoCodec(value string) string {
	upper := strings.ToUpper(value)
	switch {
	case strings.Contains(upper, "HEVC"):
		return "HEVC"
	case strings.Contains(upper, "AVC"), strings.Contains(upper, "H.264"):
		return "AVC"
	case strings.Contains(upper, "VC-1"):
		return "VC-1"
	case strings.Contains(upper, "MPEG-2"):
		return "MPEG-2"
	default:
		return strings.TrimSpace(value)
	}
}
