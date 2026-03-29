package remux

func BuildMKVMergeArgs(draft Draft) []string {
	args := []string{"--output", draft.OutputPath}

	if draft.Playlist != "" {
		args = append(args, "--playlist", draft.Playlist)
	}
	if draft.EnableDV {
		args = append(args, "--engage", "enable_dolby_vision")
	}
	if draft.Video.Name != "" {
		args = append(args, "--track-name", "0:"+draft.Video.Name)
	}

	for _, track := range draft.Audio {
		if !track.Selected {
			continue
		}
		if track.Language != "" {
			args = append(args, "--language", track.ID+":"+track.Language)
		}
		if track.Name != "" {
			args = append(args, "--track-name", track.ID+":"+track.Name)
		}
		if track.Default {
			args = append(args, "--default-track", track.ID+":yes")
			continue
		}
		args = append(args, "--default-track", track.ID+":no")
	}

	if draft.SourcePath != "" {
		args = append(args, draft.SourcePath)
	}

	return args
}
