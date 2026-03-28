package analyzer

type PlaylistInfo struct {
	Name               string `json:"name"`
	DurationSeconds    int    `json:"durationSeconds"`
	SizeBytes          int64  `json:"sizeBytes"`
	ChapterCount       int    `json:"chapterCount"`
	VideoSummary       string `json:"videoSummary"`
	FeatureScore       int64  `json:"featureScore"`
	IsFeatureCandidate bool   `json:"isFeatureCandidate"`
}
