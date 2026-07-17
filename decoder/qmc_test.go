package decoder

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestQMC1MaskBoundaryVectors(t *testing.T) {
	tests := []struct {
		offset int64
		want   byte
	}{
		{offset: 0, want: 0xc3},
		{offset: 1, want: 0x4a},
		{offset: 0x7ffe, want: 0xd6},
		{offset: 0x7fff, want: 0x4a},
		{offset: 0x8000, want: 0x4a},
	}
	for _, test := range tests {
		if got := qmc1Mask(test.offset); got != test.want {
			t.Fatalf("qmc1Mask(%#x) = %#x, want %#x", test.offset, got, test.want)
		}
	}
}

func TestQMC1IsReversibleAcrossBoundaries(t *testing.T) {
	original := make([]byte, 0x8008)
	for i := range original {
		original[i] = byte(i * 31)
	}
	decoded := append([]byte(nil), original...)
	for pass := 0; pass < 2; pass++ {
		for i := range decoded {
			decoded[i] ^= qmc1Mask(int64(i))
		}
	}
	if !bytes.Equal(decoded, original) {
		t.Fatal("QMC1 decrypt was not reversible across the 0x7fff boundary")
	}
}

func TestQMC1ProbeRecognizesEncryptedAudio(t *testing.T) {
	plain := append([]byte("OggS"), bytes.Repeat([]byte{0x55}, 64)...)
	encrypted := append([]byte(nil), plain...)
	for i := range encrypted {
		encrypted[i] ^= qmc1Mask(int64(i))
	}
	path := filepath.Join(t.TempDir(), "sample.bkcogg")
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	if !qmc1ProbeLooksLikeAudio(path) {
		t.Fatal("QMC1 probe did not recognize encrypted OGG data")
	}
}

func TestParseMusicExFooter(t *testing.T) {
	const (
		footerSize = 0xc0
		songID     = 123456789
		mid        = "003aBcDeFgHiJk"
		filename   = "O4M0003aBcDeFgHiJk.mgg"
	)
	audio := []byte("encrypted-audio")
	data := makeMusicExData(audio, songID, mid, filename)

	footer := parseQMCFooter(data)
	if footer.Kind != qmcFooterMusicEx {
		t.Fatalf("footer kind = %v, want MusicEx", footer.Kind)
	}
	if footer.AudioLen != len(audio) || footer.FooterSize != footerSize {
		t.Fatalf("audio/footer sizes = %d/%d", footer.AudioLen, footer.FooterSize)
	}
	if footer.SongID != songID || footer.MID != mid || footer.Filename != filename {
		t.Fatalf("unexpected metadata: %#v", footer)
	}
}

func TestOptionalLocalMusicExSamples(t *testing.T) {
	directory := os.Getenv("QMC_SAMPLE_DIR")
	if directory == "" {
		t.Skip("QMC_SAMPLE_DIR is not set")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read sample directory: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !isQMC2Extension(strings.ToLower(filepath.Ext(entry.Name()))) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatalf("read sample: %v", err)
		}
		footer := parseQMCFooter(data)
		if footer.Kind != qmcFooterMusicEx {
			t.Fatalf("sample %q footer kind = %v, want MusicEx", entry.Name(), footer.Kind)
		}
		if footer.MID == "" || footer.Filename == "" || footer.AudioLen <= 0 {
			t.Fatalf("sample %q has incomplete MusicEx metadata", entry.Name())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("sample directory contains no supported QMC2 files")
	}
}

func TestMalformedQMCFootersAreRejected(t *testing.T) {
	tests := map[string][]byte{
		"musicex size": func() []byte {
			data := make([]byte, 32)
			binary.LittleEndian.PutUint32(data[len(data)-16:len(data)-12], 4096)
			binary.LittleEndian.PutUint32(data[len(data)-12:len(data)-8], 1)
			copy(data[len(data)-8:], []byte("musicex\x00"))
			return data
		}(),
		"qtag size": func() []byte {
			data := make([]byte, 24)
			binary.BigEndian.PutUint32(data[len(data)-8:len(data)-4], 4096)
			copy(data[len(data)-4:], []byte("QTag"))
			return data
		}(),
		"v1 size": func() []byte {
			data := make([]byte, 24)
			binary.LittleEndian.PutUint32(data[len(data)-4:], 512)
			return data
		}(),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if footer := parseQMCFooter(data); footer.Kind != qmcFooterUnknown {
				t.Fatalf("malformed footer detected as kind %v", footer.Kind)
			}
		})
	}
}

func TestParseSTagFooter(t *testing.T) {
	data := append([]byte("encrypted-audio"), []byte("STag")...)
	footer := parseQMCFooter(data)
	if footer.Kind != qmcFooterSTag {
		t.Fatalf("footer kind = %v, want STag", footer.Kind)
	}
	if footer.AudioLen != len(data) {
		t.Fatalf("STag audio length = %d, want %d", footer.AudioLen, len(data))
	}
}

func TestParseQMC2EKeyVector(t *testing.T) {
	const encoded = "VGhpcyBpcyBHFWEh4cjZ1Vi7rJ56XeoPlqGM1sxBGPg7mt89umKclFBr9iqfmFdS"
	key, err := parseQMC2EKey(encoded)
	if err != nil {
		t.Fatalf("parseQMC2EKey() error = %v", err)
	}
	const want = "This is a test key for test purpose :D"
	if string(key) != want {
		t.Fatalf("key = %q, want %q", string(key), want)
	}
}

func TestQMC2V1AcceptsRawAndBase64KeyTails(t *testing.T) {
	key := []byte("12345678")
	plain := append([]byte("OggS"), bytes.Repeat([]byte{0x42}, 128)...)
	encrypted := append([]byte(nil), plain...)
	(&qmc2MapCipher{key: key}).decrypt(0, encrypted)

	for name, keyTail := range map[string][]byte{
		"raw":    key,
		"base64": []byte(base64.StdEncoding.EncodeToString(key)),
	} {
		t.Run(name, func(t *testing.T) {
			data := append(append([]byte(nil), encrypted...), keyTail...)
			var size [4]byte
			binary.LittleEndian.PutUint32(size[:], uint32(len(keyTail)))
			data = append(data, size[:]...)
			got, err := decryptQMC2Data(context.Background(), data, "sample.mgg1", DecodeOptions{}, nil)
			if err != nil {
				t.Fatalf("decryptQMC2Data() error = %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatal("V1 decrypted data does not match plaintext")
			}
		})
	}
}

func TestQMC2MapVector(t *testing.T) {
	key := []byte("ABCDEFGHIJKLMNOP")
	data := make([]byte, 16)
	(&qmc2MapCipher{key: key}).decrypt(0, data)
	want := []byte{
		0x3f, 0x8a, 0xc1, 0x49, 0x3f, 0x49, 0xc1, 0x8a,
		0x3f, 0x8a, 0xc1, 0x49, 0x3f, 0x49, 0xc1, 0x8a,
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("map vector = %x, want %x", data, want)
	}
}

func TestQMC2MapBoundaryVector(t *testing.T) {
	key := []byte("ABCDEFGHIJKLMNOP")
	data := make([]byte, 16)
	(&qmc2MapCipher{key: key}).decrypt(0x7fff-8, data)
	want := []byte{
		0x8a, 0x3f, 0x8a, 0xc1, 0x49, 0x3f, 0x49, 0xc1,
		0x8a, 0x8a, 0xc1, 0x49, 0x3f, 0x49, 0xc1, 0x8a,
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("map boundary vector = %x, want %x", data, want)
	}
}

func TestQMC2RC4FirstSegmentVector(t *testing.T) {
	key := make([]byte, 255)
	for i := range key {
		key[i] = byte(i)
	}
	data := make([]byte, 16)
	newQMC2RC4Cipher(key).decrypt(0, data)
	want := []byte{0, 50, 16, 8, 5, 3, 2, 1, 1, 1, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(data, want) {
		t.Fatalf("RC4 first segment = %x, want %x", data, want)
	}
}

func TestQMC2ChunkedDecryptMatchesWholeBuffer(t *testing.T) {
	mapKey := []byte("a deterministic map key")
	rc4Key := make([]byte, 512)
	for i := range rc4Key {
		rc4Key[i] = byte((i*17)%251 + 1)
	}
	for name, cipherFactory := range map[string]func() qmc2StreamCipher{
		"map": func() qmc2StreamCipher { return &qmc2MapCipher{key: append([]byte(nil), mapKey...)} },
		"rc4": func() qmc2StreamCipher { return newQMC2RC4Cipher(rc4Key) },
	} {
		t.Run(name, func(t *testing.T) {
			original := make([]byte, 40000)
			for i := range original {
				original[i] = byte(i*13 + 7)
			}
			whole := append([]byte(nil), original...)
			cipherFactory().decrypt(0, whole)

			chunked := append([]byte(nil), original...)
			cipher := cipherFactory()
			boundaries := []int{1, 127, 128, 129, 5119, 5120, 5121, 8193, len(chunked)}
			start := 0
			for _, end := range boundaries {
				if end <= start || end > len(chunked) {
					continue
				}
				cipher.decrypt(start, chunked[start:end])
				start = end
			}
			if start < len(chunked) {
				cipher.decrypt(start, chunked[start:])
			}
			if !bytes.Equal(chunked, whole) {
				t.Fatal("chunked decryption differs from whole-buffer decryption")
			}
		})
	}
}

func TestDecryptQMC2FileStreamsMapAndRC4(t *testing.T) {
	mapKey := []byte("streaming-map-key")
	rc4Key := make([]byte, 513)
	for index := range rc4Key {
		rc4Key[index] = byte((index*29)%251 + 1)
	}

	for _, test := range []struct {
		name string
		key  []byte
	}{
		{name: "map", key: mapKey},
		{name: "rc4", key: rc4Key},
	} {
		t.Run(test.name, func(t *testing.T) {
			plain := make([]byte, 2*1024*1024+12345)
			copy(plain, []byte("fLaC"))
			for index := 4; index < len(plain); index++ {
				plain[index] = byte(index*31 + 17)
			}
			encrypted := append([]byte(nil), plain...)
			cipher, err := newQMC2Cipher(test.key)
			if err != nil {
				t.Fatal(err)
			}
			const encryptChunk = 333333
			for start := 0; start < len(encrypted); start += encryptChunk {
				end := start + encryptChunk
				if end > len(encrypted) {
					end = len(encrypted)
				}
				cipher.decrypt(start, encrypted[start:end])
			}

			directory := t.TempDir()
			inputPath := filepath.Join(directory, "sample.mflac")
			outputPath := filepath.Join(directory, "sample.raw")
			if err := os.WriteFile(inputPath, makeMusicExData(encrypted, 1, "song-mid", "sample.mflac"), 0600); err != nil {
				t.Fatal(err)
			}
			lastProgress := 0
			err = decryptQMC2File(
				context.Background(),
				inputPath,
				outputPath,
				DecodeOptions{QQEKey: base64.StdEncoding.EncodeToString(test.key)},
				func(percent int) { lastProgress = percent },
			)
			if err != nil {
				t.Fatalf("decryptQMC2File() error = %v", err)
			}
			got, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatal("streamed output does not match plaintext")
			}
			if lastProgress != 90 {
				t.Fatalf("last stream progress = %d, want 90", lastProgress)
			}
		})
	}
}

func TestDecryptQMC2FileCancellationRemovesPartialOutput(t *testing.T) {
	key := []byte("streaming-map-key")
	plain := append([]byte("fLaC"), bytes.Repeat([]byte{0x2a}, 1024)...)
	encrypted := append([]byte(nil), plain...)
	(&qmc2MapCipher{key: key}).decrypt(0, encrypted)

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "sample.mflac")
	outputPath := filepath.Join(directory, "sample.raw")
	if err := os.WriteFile(inputPath, makeMusicExData(encrypted, 1, "song-mid", "sample.mflac"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := decryptQMC2File(
		ctx,
		inputPath,
		outputPath,
		DecodeOptions{QQEKey: base64.StdEncoding.EncodeToString(key)},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "任务已取消") {
		t.Fatalf("decryptQMC2File() cancellation error = %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("partial streamed output was not removed: %v", statErr)
	}
}

func putUTF16LE(target []byte, offset int, maxBytes int, value string) {
	units := utf16.Encode([]rune(value))
	end := offset + maxBytes
	for i, unit := range units {
		position := offset + i*2
		if position+1 >= end || position+1 >= len(target) {
			break
		}
		binary.LittleEndian.PutUint16(target[position:position+2], unit)
	}
}

func makeMusicExData(audio []byte, songID uint32, mid string, filename string) []byte {
	const footerSize = 0xc0
	meta := make([]byte, footerSize-16)
	binary.LittleEndian.PutUint32(meta[0:4], songID)
	putUTF16LE(meta, 0x0c, 60, mid)
	putUTF16LE(meta, 0x48, 68, filename)
	data := append(append([]byte(nil), audio...), meta...)
	var trailer [8]byte
	binary.LittleEndian.PutUint32(trailer[0:4], footerSize)
	binary.LittleEndian.PutUint32(trailer[4:8], 1)
	data = append(data, trailer[:]...)
	return append(data, []byte("musicex\x00")...)
}
