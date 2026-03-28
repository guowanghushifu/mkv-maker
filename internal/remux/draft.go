package remux

type Draft struct {
	Title string
	Video VideoTrack
	Audio []AudioTrack
}

type VideoTrack struct {
	Resolution string
	Codec      string
	HDRType    string
}

type AudioTrack struct {
	Name       string
	CodecLabel string
	Default    bool
	Selected   bool
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
