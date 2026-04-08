package remux

import (
	"strings"
	"unicode"

	"github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
)

func BuildFilename(d Draft) string {
	audioLabel := "UnknownAudio"
	if track, ok := d.DefaultSelectedAudio(); ok && strings.TrimSpace(track.CodecLabel) != "" {
		audioLabel = normalizeFilenameAudioCodecLabel(track.CodecLabel)
	}

	hdrLabel := d.Video.HDRType
	if d.EnableDV && !strings.Contains(strings.ToUpper(hdrLabel), "DV") {
		hdrLabel = "DV.HDR"
	}
	if strings.EqualFold(strings.TrimSpace(hdrLabel), "HDR.DV") {
		hdrLabel = "DV.HDR"
	}

	parts := []string{
		d.Title + " - " + d.Video.Resolution,
		"BluRay",
		hdrLabel,
		d.Video.Codec,
		audioLabel,
	}
	return sanitizeFilename(strings.Join(compact(parts), ".")) + ".mkv"
}

func normalizeFilenameAudioCodecLabel(value string) string {
	normalized := bdinfo.NormalizeAudioCodecLabel(value)
	if hasNormalizedAudioCodecBase(normalized) {
		return normalized
	}

	aliasNormalized := bdinfo.NormalizeAudioCodecLabel(strings.ReplaceAll(value, "_", "-"))
	if hasNormalizedAudioCodecBase(aliasNormalized) {
		return aliasNormalized
	}
	return value
}

func hasNormalizedAudioCodecBase(value string) bool {
	for _, prefix := range []string{"TrueHD", "DTS-HD.MA", "DTS-HD", "DDP", "DD", "LPCM", "AAC"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
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

func sanitizeFilename(value string) string {
	builder := strings.Builder{}
	lastDot := false
	lastSpace := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '(' || r == ')':
			builder.WriteRune(r)
			lastDot = false
			lastSpace = false
		case r == '.' || r == '+' || r == '-' || r == '_':
			out := r
			if r == '_' {
				out = '.'
			}
			if !lastDot && builder.Len() > 0 {
				builder.WriteRune(out)
				lastDot = out == '.'
				lastSpace = false
			}
		case unicode.IsSpace(r):
			if !lastSpace && builder.Len() > 0 {
				builder.WriteRune(' ')
				lastSpace = true
				lastDot = false
			}
		}
	}

	return strings.Trim(builder.String(), " .")
}
