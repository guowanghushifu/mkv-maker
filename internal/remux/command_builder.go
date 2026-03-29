package remux

import (
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func BuildMKVMergeArgs(d Draft) []string {
	args := []string{"--output", d.OutputPath}
	audioSelectors := make([]string, 0, len(d.Audio))
	subtitleSelectors := make([]string, 0, len(d.Subtitles))
	trackOrder := []string{"0:0"}

	if d.Video.Name != "" {
		args = append(args, "--track-name", "0:"+d.Video.Name)
	}

	for index, track := range d.Audio {
		if !track.Selected {
			continue
		}

		audioSelector := resolveTrackSelector(track.ID, index)
		audioSelectors = append(audioSelectors, audioSelector)
		trackOrder = append(trackOrder, "0:"+audioSelector)

		args = append(args, "--language", audioSelector+":"+track.Language)
		args = append(args, "--track-name", audioSelector+":"+track.Name)
		defaultValue := "no"
		if track.Default {
			defaultValue = "yes"
		}
		args = append(args, "--default-track-flag", audioSelector+":"+defaultValue)
	}

	if len(audioSelectors) > 0 {
		args = append(args, "--audio-tracks", strings.Join(audioSelectors, ","))
	}

	for index, track := range d.Subtitles {
		if !track.Selected {
			continue
		}

		subtitleSelector := resolveTrackSelector(track.ID, index)
		subtitleSelectors = append(subtitleSelectors, subtitleSelector)
		trackOrder = append(trackOrder, "0:"+subtitleSelector)

		args = append(args, "--language", subtitleSelector+":"+track.Language)
		args = append(args, "--track-name", subtitleSelector+":"+track.Name)
		defaultValue := "no"
		if track.Default {
			defaultValue = "yes"
		}
		args = append(args, "--default-track-flag", subtitleSelector+":"+defaultValue)
		if track.Forced {
			args = append(args, "--forced-display-flag", subtitleSelector+":yes")
		}
	}

	if len(subtitleSelectors) > 0 {
		args = append(args, "--subtitle-tracks", strings.Join(subtitleSelectors, ","))
	}
	args = append(args, "--track-order", strings.Join(trackOrder, ","))

	args = append(args, resolveInputPaths(d)...)
	return args
}

func resolveTrackSelector(trackID string, index int) string {
	trimmed := strings.TrimSpace(trackID)
	if trimmed != "" {
		if _, err := strconv.Atoi(trimmed); err == nil {
			return trimmed
		}

		digits := strings.Builder{}
		for _, r := range trimmed {
			if unicode.IsDigit(r) {
				digits.WriteRune(r)
			}
		}
		if digits.Len() > 0 {
			return digits.String()
		}
	}
	return strconv.Itoa(index + 1)
}

func resolveInputPaths(d Draft) []string {
	if len(d.SegmentPaths) > 0 {
		args := make([]string, 0, len(d.SegmentPaths)*2-1)
		for i, path := range d.SegmentPaths {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if len(args) > 0 {
				args = append(args, "+")
			}
			args = append(args, path)
			if i == len(d.SegmentPaths)-1 {
				break
			}
		}
		if len(args) > 0 {
			return args
		}
	}

	sourcePath := strings.TrimSpace(d.SourcePath)
	if sourcePath == "" {
		return nil
	}
	if strings.EqualFold(filepath.Ext(sourcePath), ".MPLS") {
		return []string{sourcePath}
	}
	if d.Playlist == "" {
		return []string{sourcePath}
	}
	playlist := strings.TrimSpace(d.Playlist)

	if strings.EqualFold(filepath.Base(sourcePath), "BDMV") {
		return []string{filepath.Join(sourcePath, "PLAYLIST", playlist)}
	}

	return []string{filepath.Join(sourcePath, "BDMV", "PLAYLIST", playlist)}
}
