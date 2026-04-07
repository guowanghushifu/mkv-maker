package remux

import (
	"regexp"
	"strconv"
	"strings"
)

var progressPercentPattern = regexp.MustCompile(`(?i)(?:^|\s)(?:progress:|#GUI#progress)\s*([0-9]{1,3})%`)
var makeMKVTotalProgressPattern = regexp.MustCompile(`(?i)(?:^|,\s*)total progress\s*-\s*([0-9]{1,3})%`)

const makeMKVProgressWeight = 60
const mkvmergeProgressWeight = 40

func FormatCommandPreview(binary string, args []string) string {
	trimmedBinary := strings.TrimSpace(binary)
	if trimmedBinary == "" {
		trimmedBinary = "mkvmerge"
	}

	lines := make([]string, 1, len(args)+1)
	lines[0] = trimmedBinary
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") && i+1 < len(args) {
			next := args[i+1]
			if next != "" && next != "+" && !strings.HasPrefix(next, "--") {
				lines = append(lines, "  "+arg+" "+next)
				i++
				continue
			}
		}
		lines = append(lines, "  "+arg)
	}
	return strings.Join(lines, "\n")
}

func ExtractProgressPercent(line string) (int, bool) {
	matches := progressPercentPattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return 0, false
	}
	return clampPercentMatch(matches[1])
}

func extractProgressPercentsFromChunk(remainder, chunk string) ([]int, string) {
	combined := remainder + chunk
	if combined == "" {
		return nil, ""
	}

	lastTerminator := strings.LastIndexAny(combined, "\r\n")
	if lastTerminator < 0 {
		return nil, combined
	}

	parseable := combined[:lastTerminator+1]
	nextRemainder := combined[lastTerminator+1:]
	parts := strings.FieldsFunc(parseable, func(r rune) bool {
		return r == '\n' || r == '\r'
	})

	percents := make([]int, 0, len(parts))
	makeMKVSaving := false
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if strings.EqualFold(trimmed, "Current action: Saving to MKV file") {
			makeMKVSaving = true
			continue
		}
		if progress, ok := extractMakeMKVProgressPercent(trimmed, makeMKVSaving); ok {
			percents = append(percents, progress)
			continue
		}
		if progress, ok := ExtractProgressPercent(trimmed); ok {
			percents = append(percents, scaleMkvmergeProgress(progress))
		}
	}
	return percents, nextRemainder
}

func extractMakeMKVProgressPercent(line string, saving bool) (int, bool) {
	if !saving {
		return 0, false
	}
	matches := makeMKVTotalProgressPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) < 2 {
		return 0, false
	}
	value, ok := clampPercentMatch(matches[1])
	if !ok {
		return 0, false
	}
	return scaleMakeMKVProgress(value), true
}

func clampPercentMatch(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	if parsed < 0 {
		parsed = 0
	}
	if parsed > 100 {
		parsed = 100
	}
	return parsed, true
}

func scaleMakeMKVProgress(progress int) int {
	return (progress * makeMKVProgressWeight) / 100
}

func scaleMkvmergeProgress(progress int) int {
	return makeMKVProgressWeight + (progress*mkvmergeProgressWeight)/100
}
