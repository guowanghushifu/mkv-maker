package analyzer

import "testing"

func TestRankPlaylistsMarksLongestAsFeatureCandidate(t *testing.T) {
	playlists := []PlaylistInfo{
		{Name: "00001.MPLS", DurationSeconds: 600, SizeBytes: 1_000},
		{Name: "00800.MPLS", DurationSeconds: 7200, SizeBytes: 30_000},
	}

	ranked := RankPlaylists(playlists)
	if !ranked[0].IsFeatureCandidate || ranked[0].Name != "00800.MPLS" {
		t.Fatalf("expected 00800.MPLS to be the top feature candidate, got %+v", ranked[0])
	}
}

func TestRankPlaylistsClearsExistingFeatureCandidateFlags(t *testing.T) {
	playlists := []PlaylistInfo{
		{Name: "00001.MPLS", DurationSeconds: 600, SizeBytes: 1_000, IsFeatureCandidate: true},
		{Name: "00800.MPLS", DurationSeconds: 7200, SizeBytes: 30_000, IsFeatureCandidate: true},
	}

	ranked := RankPlaylists(playlists)
	count := 0
	for _, playlist := range ranked {
		if playlist.IsFeatureCandidate {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one feature candidate, got %d", count)
	}
	if ranked[0].Name != "00800.MPLS" || !ranked[0].IsFeatureCandidate {
		t.Fatalf("expected 00800.MPLS to be the only feature candidate, got %+v", ranked)
	}
}
