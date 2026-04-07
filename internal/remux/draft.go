package remux

type Draft struct {
	Title             string
	SourcePath        string
	MakeMKVSourcePath string
	Playlist          string
	OutputPath        string
	EnableDV          bool
	SegmentPaths      []string
	Video             VideoTrack
	Audio             []AudioTrack
	Subtitles         []SubtitleTrack
}

type VideoTrack struct {
	Name       string
	Resolution string
	Codec      string
	HDRType    string
}

type AudioTrack struct {
	ID          string
	Name        string
	Language    string
	CodecLabel  string
	SourceIndex int
	Default     bool
	Selected    bool
}

type SubtitleTrack struct {
	ID          string
	Name        string
	Language    string
	SourceIndex int
	Default     bool
	Selected    bool
	Forced      bool
}

func (d Draft) DefaultSelectedAudio() (AudioTrack, bool) {
	for _, track := range d.Audio {
		if track.Selected && track.Default {
			return track, true
		}
	}
	for _, track := range d.Audio {
		if track.Selected {
			return track, true
		}
	}
	for _, track := range d.Audio {
		if track.Default {
			return track, true
		}
	}
	return AudioTrack{}, false
}
