package remux

import "strconv"

func BuildMKVMergeArgs(d Draft) []string {
	args := []string{"--output", d.OutputPath}

	if d.Video.Name != "" {
		args = append(args, "--track-name", "0:"+d.Video.Name)
	}

	for index, track := range d.Audio {
		audioSelector := strconv.Itoa(index + 1)

		args = append(args, "--language", audioSelector+":"+track.Language)
		args = append(args, "--track-name", audioSelector+":"+track.Name)
		if track.Selected && track.Default {
			args = append(args, "--default-track-flag", audioSelector+":yes")
		}
	}

	if d.EnableDV {
		args = append(args, "--engage", "merge_dolby_vision")
	}

	args = append(args, d.SourcePath)
	return args
}
