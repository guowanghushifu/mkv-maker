package analyzer

import "slices"

func RankPlaylists(in []PlaylistInfo) []PlaylistInfo {
	out := append([]PlaylistInfo(nil), in...)
	for i := range out {
		out[i].FeatureScore = int64(out[i].DurationSeconds)*1000 + out[i].SizeBytes + int64(out[i].ChapterCount)*100
	}
	slices.SortFunc(out, func(a, b PlaylistInfo) int {
		switch {
		case a.FeatureScore > b.FeatureScore:
			return -1
		case a.FeatureScore < b.FeatureScore:
			return 1
		default:
			return 0
		}
	})

	if len(out) > 0 {
		out[0].IsFeatureCandidate = true
	}

	return out
}
