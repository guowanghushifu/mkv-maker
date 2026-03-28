package remux

import "strings"

func BuildFilename(d Draft) string {
	audioLabel := "UnknownAudio"
	for _, track := range d.Audio {
		if track.Selected && track.Default {
			audioLabel = track.CodecLabel
			break
		}
	}

	parts := []string{
		d.Title + " - " + d.Video.Resolution,
		"BluRay",
		d.Video.HDRType,
		d.Video.Codec,
		audioLabel,
	}
	return strings.Join(compact(parts), ".") + ".mkv"
}

func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}
