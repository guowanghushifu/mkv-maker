package remux

import (
	"regexp"
	"strconv"
	"strings"
)

var progressPercentPattern = regexp.MustCompile(`(?i)(?:^|\s)(?:progress:|#GUI#progress)\s*([0-9]{1,3})%`)

func FormatCommandPreview(binary string, args []string) string {
	_ = binary

	lines := make([]string, 1, len(args)+1)
	lines[0] = "mkvmerge"
	for _, arg := range args {
		lines = append(lines, "  "+arg)
	}
	return strings.Join(lines, "\n")
}

func ExtractProgressPercent(line string) (int, bool) {
	matches := progressPercentPattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return 0, false
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value, true
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
	for _, part := range parts {
		if progress, ok := ExtractProgressPercent(part); ok {
			percents = append(percents, progress)
		}
	}
	return percents, nextRemainder
}
