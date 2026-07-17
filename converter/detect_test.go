package converter

import "testing"

func TestDetectAudioExtFromHeader(t *testing.T) {
	tests := []struct {
		name   string
		header []byte
		want   string
	}{
		{name: "flac", header: []byte("fLaC"), want: ".flac"},
		{name: "ogg", header: []byte("OggS"), want: ".ogg"},
		{name: "m4a", header: []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p'}, want: ".m4a"},
		{name: "ape", header: []byte("MAC "), want: ".ape"},
		{name: "wma", header: append([]byte(nil), asfHeader...), want: ".wma"},
		{name: "aac", header: []byte{0xff, 0xf1, 0x50, 0x80}, want: ".aac"},
		{name: "mp3 frame", header: []byte{0xff, 0xfb, 0x90, 0x64}, want: ".mp3"},
		{name: "unknown", header: []byte("nope"), want: ".bin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DetectAudioExtFromHeader(test.header); got != test.want {
				t.Fatalf("DetectAudioExtFromHeader() = %q, want %q", got, test.want)
			}
		})
	}
}
