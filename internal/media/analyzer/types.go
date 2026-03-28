package analyzer

type PlaylistInfo struct {
	Name               string
	DurationSeconds    int
	SizeBytes          int64
	ChapterCount       int
	FeatureScore       float64
	IsFeatureCandidate bool
}
