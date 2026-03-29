package mpls

import (
	"encoding/binary"
	"testing"
)

func TestParseClipNamesBytesExtractsPlayItems(t *testing.T) {
	data := make([]byte, 64)
	copy(data[:4], []byte("MPLS"))
	copy(data[4:8], []byte("0300"))
	binary.BigEndian.PutUint32(data[8:12], 20)

	playlistStart := 20
	binary.BigEndian.PutUint32(data[playlistStart:playlistStart+4], 0)
	binary.BigEndian.PutUint16(data[playlistStart+4:playlistStart+6], 0)
	binary.BigEndian.PutUint16(data[playlistStart+6:playlistStart+8], 2)
	binary.BigEndian.PutUint16(data[playlistStart+8:playlistStart+10], 0)

	cursor := playlistStart + 10
	binary.BigEndian.PutUint16(data[cursor:cursor+2], 9)
	copy(data[cursor+2:cursor+7], []byte("00005"))
	copy(data[cursor+7:cursor+11], []byte("M2TS"))
	cursor += 11

	binary.BigEndian.PutUint16(data[cursor:cursor+2], 9)
	copy(data[cursor+2:cursor+7], []byte("00006"))
	copy(data[cursor+7:cursor+11], []byte("M2TS"))

	clips, err := ParseClipNamesBytes(data)
	if err != nil {
		t.Fatalf("ParseClipNamesBytes returned error: %v", err)
	}
	if len(clips) != 2 || clips[0] != "00005" || clips[1] != "00006" {
		t.Fatalf("unexpected clips: %+v", clips)
	}
}

func TestParseClipNamesBytesRejectsInvalidHeader(t *testing.T) {
	if _, err := ParseClipNamesBytes([]byte("not-mpls")); err == nil {
		t.Fatal("expected invalid header error")
	}
}
