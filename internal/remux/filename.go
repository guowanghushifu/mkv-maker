package remux

import "strings"

func BuildFilename(d Draft) string {
	audioLabel := "UnknownAudio"
	if track, ok := d.DefaultSelectedAudio(); ok && strings.TrimSpace(track.CodecLabel) != "" {
		audioLabel = track.CodecLabel
	}

	hdrLabel := d.Video.HDRType
	if d.EnableDV && !strings.Contains(strings.ToUpper(hdrLabel), "DV") {
		hdrLabel = "HDR.DV"
	}

	parts := []string{
		d.Title + " - " + d.Video.Resolution,
		"BluRay",
		hdrLabel,
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
