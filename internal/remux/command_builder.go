package remux

import (
	"path/filepath"
	"strconv"
	"strings"
)

func BuildMKVMergeArgs(d Draft) []string {
	args := []string{"--output", d.OutputPath}

	if d.Video.Name != "" {
		args = append(args, "--track-name", "0:"+d.Video.Name)
	}

	for index, track := range d.Audio {
		if !track.Selected {
			continue
		}

		audioSelector := resolveAudioSelector(track.ID, index)

		args = append(args, "--language", audioSelector+":"+track.Language)
		args = append(args, "--track-name", audioSelector+":"+track.Name)
		if track.Default {
			args = append(args, "--default-track-flag", audioSelector+":yes")
		}
	}

	if d.EnableDV {
		args = append(args, "--engage", "merge_dolby_vision")
	}

	args = append(args, resolveInputPath(d))
	return args
}

func resolveAudioSelector(trackID string, index int) string {
	trimmed := strings.TrimSpace(trackID)
	if trimmed != "" {
		if _, err := strconv.Atoi(trimmed); err == nil {
			return trimmed
		}
	}
	return strconv.Itoa(index + 1)
}

func resolveInputPath(d Draft) string {
	if d.Playlist == "" || d.SourcePath == "" {
		return d.SourcePath
	}

	sourcePathLower := strings.ToLower(d.SourcePath)
	if strings.HasSuffix(sourcePathLower, ".iso") {
		return d.SourcePath
	}

	if strings.EqualFold(filepath.Base(d.SourcePath), "BDMV") {
		return filepath.Join(d.SourcePath, "PLAYLIST", d.Playlist)
	}

	return filepath.Join(d.SourcePath, "BDMV", "PLAYLIST", d.Playlist)
}
