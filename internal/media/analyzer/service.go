package analyzer

import "sort"

func RankPlaylists(playlists []PlaylistInfo) []PlaylistInfo {
	ranked := make([]PlaylistInfo, len(playlists))
	copy(ranked, playlists)

	for i := range ranked {
		ranked[i].FeatureScore = featureScore(ranked[i])
		ranked[i].IsFeatureCandidate = false
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].FeatureScore != ranked[j].FeatureScore {
			return ranked[i].FeatureScore > ranked[j].FeatureScore
		}
		if ranked[i].DurationSeconds != ranked[j].DurationSeconds {
			return ranked[i].DurationSeconds > ranked[j].DurationSeconds
		}
		if ranked[i].SizeBytes != ranked[j].SizeBytes {
			return ranked[i].SizeBytes > ranked[j].SizeBytes
		}
		return ranked[i].Name < ranked[j].Name
	})

	if len(ranked) > 0 {
		ranked[0].IsFeatureCandidate = true
	}

	return ranked
}

func featureScore(info PlaylistInfo) float64 {
	return float64(info.DurationSeconds) +
		(float64(info.SizeBytes) / 1_000_000.0) +
		(float64(info.ChapterCount) * 120.0)
}
