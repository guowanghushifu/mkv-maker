package bdinfo

import (
	"bufio"
	"errors"
	"strings"
)

var ErrNoRecognizedFields = errors.New("no recognized bdinfo fields")

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
	foundField := false
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "PLAYLIST:"):
			parsed.PlaylistName = strings.TrimSpace(strings.TrimPrefix(line, "PLAYLIST:"))
			foundField = true
		case strings.HasPrefix(line, "VIDEO:"):
			parsed.VideoTracks = append(parsed.VideoTracks, TrackLabel{
				RawLine: line,
				Name:    strings.TrimSpace(strings.TrimPrefix(line, "VIDEO:")),
			})
			foundField = true
		case strings.HasPrefix(line, "AUDIO:"):
			parsed.AudioTracks = append(parsed.AudioTracks, parseTrack("AUDIO:", line))
			foundField = true
		case strings.HasPrefix(line, "SUBTITLE:"):
			parsed.SubtitleTracks = append(parsed.SubtitleTracks, parseTrack("SUBTITLE:", line))
			foundField = true
		}
	}

	if err := scanner.Err(); err != nil {
		return Parsed{}, err
	}
	if !foundField {
		return Parsed{}, ErrNoRecognizedFields
	}

	return parsed, nil
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
