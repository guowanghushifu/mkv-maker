package remux

import (
	"fmt"
	"path/filepath"
	"strings"
)

func BuildMKVMergeArgs(d Draft) []string {
	args, err := BuildMKVMergeArgsWithResolvedSelectors(d, selectorsFromAudioTracks(d.Audio), selectorsFromSubtitleTracks(d.Subtitles))
	if err == nil {
		return args
	}
	return []string{"--output", d.OutputPath}
}

func BuildMKVMergeArgsWithResolvedSelectors(d Draft, audioSelectors, subtitleSelectors []ResolvedTrackSelector) ([]string, error) {
	resolvedAudio := make(map[int]string, len(audioSelectors))
	for _, selector := range audioSelectors {
		resolvedAudio[selector.SourceIndex] = selector.TrackID
	}
	resolvedSubtitles := make(map[int]string, len(subtitleSelectors))
	for _, selector := range subtitleSelectors {
		resolvedSubtitles[selector.SourceIndex] = selector.TrackID
	}

	args := []string{"--output", d.OutputPath}
	audioTrackIDs := make([]string, 0, len(d.Audio))
	subtitleTrackIDs := make([]string, 0, len(d.Subtitles))
	trackOrder := []string{"0:0"}
	if d.Video.Name != "" {
		args = append(args, "--track-name", "0:"+d.Video.Name)
	}

	for _, track := range d.Audio {
		if !track.Selected {
			continue
		}
		audioSelector, ok := resolvedAudio[track.SourceIndex]
		if !ok {
			return nil, fmt.Errorf("audio sourceIndex %d has no resolved track id", track.SourceIndex)
		}
		audioTrackIDs = append(audioTrackIDs, audioSelector)
		trackOrder = append(trackOrder, "0:"+audioSelector)
		args = append(args, "--language", audioSelector+":"+track.Language)
		args = append(args, "--track-name", audioSelector+":"+track.Name)
		defaultValue := "no"
		if track.Default {
			defaultValue = "yes"
		}
		args = append(args, "--default-track-flag", audioSelector+":"+defaultValue)
	}

	for _, track := range d.Subtitles {
		if !track.Selected {
			continue
		}
		subtitleSelector, ok := resolvedSubtitles[track.SourceIndex]
		if !ok {
			return nil, fmt.Errorf("subtitle sourceIndex %d has no resolved track id", track.SourceIndex)
		}
		subtitleTrackIDs = append(subtitleTrackIDs, subtitleSelector)
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

	if len(audioTrackIDs) > 0 {
		args = append(args, "--audio-tracks", strings.Join(audioTrackIDs, ","))
	}
	if len(subtitleTrackIDs) > 0 {
		args = append(args, "--subtitle-tracks", strings.Join(subtitleTrackIDs, ","))
	}
	args = append(args, "--track-order", strings.Join(trackOrder, ","))
	if inputPath := resolveInputPath(d); inputPath != "" {
		args = append(args, inputPath)
	}
	return args, nil
}

func selectorsFromAudioTracks(tracks []AudioTrack) []ResolvedTrackSelector {
	selectors := make([]ResolvedTrackSelector, 0, len(tracks))
	for _, track := range tracks {
		selectors = append(selectors, ResolvedTrackSelector{SourceIndex: track.SourceIndex, TrackID: resolveTrackSelector(track.SourceIndex)})
	}
	return selectors
}

func selectorsFromSubtitleTracks(tracks []SubtitleTrack) []ResolvedTrackSelector {
	selectors := make([]ResolvedTrackSelector, 0, len(tracks))
	for _, track := range tracks {
		selectors = append(selectors, ResolvedTrackSelector{SourceIndex: track.SourceIndex, TrackID: resolveTrackSelector(track.SourceIndex)})
	}
	return selectors
}

func resolveTrackSelector(sourceIndex int) string {
	if sourceIndex < 0 {
		return "0"
	}
	return fmt.Sprintf("%d", sourceIndex)
}

func resolveInputPath(d Draft) string {
	sourcePath := strings.TrimSpace(d.SourcePath)
	if sourcePath == "" {
		return ""
	}
	ext := filepath.Ext(sourcePath)
	if strings.EqualFold(ext, ".MPLS") || strings.EqualFold(ext, ".MKV") {
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
