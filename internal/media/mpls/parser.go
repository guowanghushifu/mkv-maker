package mpls

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrInvalidHeader   = errors.New("invalid mpls header")
	ErrInvalidPlaylist = errors.New("invalid mpls playlist section")
)

func ParseClipNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseClipNamesBytes(data)
}

func ParseClipNamesBytes(data []byte) ([]string, error) {
	if len(data) < 20 || string(data[:4]) != "MPLS" {
		return nil, ErrInvalidHeader
	}

	playlistStart := int(binary.BigEndian.Uint32(data[8:12]))
	if playlistStart < 0 || playlistStart+10 > len(data) {
		return nil, ErrInvalidPlaylist
	}

	playItemCount := int(binary.BigEndian.Uint16(data[playlistStart+6 : playlistStart+8]))
	cursor := playlistStart + 10
	clips := make([]string, 0, playItemCount)

	for i := 0; i < playItemCount; i++ {
		if cursor+7 > len(data) {
			return nil, fmt.Errorf("%w: play item %d truncated", ErrInvalidPlaylist, i)
		}
		itemLength := int(binary.BigEndian.Uint16(data[cursor : cursor+2]))
		itemStart := cursor + 2
		itemEnd := itemStart + itemLength
		if itemEnd > len(data) || itemLength < 9 {
			return nil, fmt.Errorf("%w: play item %d has invalid length", ErrInvalidPlaylist, i)
		}

		clipName := strings.TrimSpace(string(data[itemStart : itemStart+5]))
		if clipName != "" {
			clips = append(clips, clipName)
		}
		cursor = itemEnd
	}

	if len(clips) == 0 {
		return nil, ErrInvalidPlaylist
	}
	return clips, nil
}
