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

	if d.EnableDV {
		args = append(args, "--engage", "merge_dolby_vision")
	}

	args = append(args, resolveInputPath(d))
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

func resolveInputPath(d Draft) string {
	sourcePath := strings.TrimSpace(d.SourcePath)
	if sourcePath == "" {
		return sourcePath
	}
	if strings.EqualFold(filepath.Ext(sourcePath), ".MPLS") {
		return sourcePath
	}
	if d.Playlist == "" {
		return sourcePath
	}
	playlist := strings.TrimSpace(d.Playlist)

	if strings.EqualFold(filepath.Base(sourcePath), "BDMV") {
		return filepath.Join(sourcePath, "PLAYLIST", playlist)
	}

	return filepath.Join(sourcePath, "BDMV", "PLAYLIST", playlist)
}
