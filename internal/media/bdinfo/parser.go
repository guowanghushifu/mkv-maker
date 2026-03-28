package bdinfo

import (
	"bufio"
	"strings"
)

type Parsed struct {
	Title          string       `json:"title"`
	PlaylistName   string       `json:"playlistName"`
	VideoTracks    []TrackLabel `json:"videoTracks"`
	AudioTracks    []TrackLabel `json:"audioTracks"`
	SubtitleTracks []TrackLabel `json:"subtitleTracks"`
}

type TrackLabel struct {
	RawLine  string `json:"rawLine"`
	Language string `json:"language"`
	Name     string `json:"name"`
}

func Parse(input string) (Parsed, error) {
	var parsed Parsed
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "PLAYLIST:"):
			parsed.PlaylistName = strings.TrimSpace(strings.TrimPrefix(line, "PLAYLIST:"))
		case strings.HasPrefix(line, "VIDEO:"):
			parsed.VideoTracks = append(parsed.VideoTracks, TrackLabel{
				RawLine: line,
				Name:    strings.TrimSpace(strings.TrimPrefix(line, "VIDEO:")),
			})
		case strings.HasPrefix(line, "AUDIO:"):
			parsed.AudioTracks = append(parsed.AudioTracks, parseTrack("AUDIO:", line))
		case strings.HasPrefix(line, "SUBTITLE:"):
			parsed.SubtitleTracks = append(parsed.SubtitleTracks, parseTrack("SUBTITLE:", line))
		}
	}

	return parsed, scanner.Err()
}

func parseTrack(prefix, line string) TrackLabel {
	body := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	parts := strings.Split(body, "/")
	label := TrackLabel{
		RawLine:  line,
		Language: "",
		Name:     body,
	}

	if len(parts) > 0 {
		label.Language = strings.TrimSpace(parts[0])
	}

	return label
}
