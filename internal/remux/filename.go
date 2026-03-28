package remux

import "strings"

func BuildFilename(draft Draft) string {
	title := compact(draft.Title)
	if title == "" {
		title = "Untitled"
	}

	parts := make([]string, 0, 6)
	if resolution := compact(draft.Video.Resolution); resolution != "" {
		parts = append(parts, resolution)
	}
	parts = append(parts, "BluRay")
	if hdr := compact(draft.Video.HDRType); hdr != "" {
		parts = append(parts, hdr)
	}
	if codec := compact(draft.Video.Codec); codec != "" {
		parts = append(parts, codec)
	}

	if audio, ok := draft.DefaultSelectedAudio(); ok {
		if codecLabel := compact(audio.CodecLabel); codecLabel != "" {
			parts = append(parts, codecLabel)
		}
	}

	body := strings.Join(parts, ".")
	if body == "" {
		return title + ".mkv"
	}
	return title + " - " + body + ".mkv"
}

func compact(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), ".")
}
