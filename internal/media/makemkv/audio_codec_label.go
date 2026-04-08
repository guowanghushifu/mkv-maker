package makemkv

import "strings"

func NormalizeAudioCodecLabel(parts ...string) string {
	cleaned := compactAudioCodecParts(parts...)
	if len(cleaned) == 0 {
		return ""
	}

	combined := strings.ToUpper(strings.Join(cleaned, " / "))
	resolved := make([]string, 0, 2)
	if codec := detectAudioCodecBase(cleaned[0]); codec != "" {
		resolved = append(resolved, codec)
	}
	if channels := extractChannelLayout(combined); channels != "" {
		resolved = append(resolved, channels)
	}
	return strings.Join(resolved, ".")
}

func detectAudioCodecBase(value string) string {
	normalized := normalizeAudioCodecShortname(value)
	switch normalized {
	case "DD", "AC3":
		return "DD"
	case "DDPLUS", "EAC3", "DDP":
		return "DDP"
	case "TRUEHD":
		return "TrueHD"
	case "DTSHDMA":
		return "DTS-HD.MA"
	case "DTSHD":
		return "DTS-HD"
	case "DTS":
		return "DTS"
	case "AAC":
		return "AAC"
	case "FLAC":
		return "FLAC"
	case "LPCM":
		return "LPCM"
	default:
		return ""
	}
}

func normalizeAudioCodecShortname(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", ":", "", "(", "", ")", "")
	return replacer.Replace(value)
}

func compactAudioCodecParts(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func extractChannelLayout(value string) string {
	for _, candidate := range []string{"7.1", "6.1", "5.1", "2.1", "2.0", "1.0"} {
		if strings.Contains(value, candidate) {
			return candidate
		}
	}
	switch {
	case strings.Contains(value, "STEREO"):
		return "2.0"
	case strings.Contains(value, "MONO"):
		return "1.0"
	default:
		return ""
	}
}
