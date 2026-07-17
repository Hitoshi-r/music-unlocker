package decoder

// QMC2/TC-TEA implementation is derived from the MIT-licensed music_unlock
// reference implementation; see THIRD_PARTY_NOTICES.md.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

type qmcFooterKind int

const (
	qmcFooterUnknown qmcFooterKind = iota
	qmcFooterV1
	qmcFooterQTag
	qmcFooterSTag
	qmcFooterMusicEx
)

type qmcFooter struct {
	Kind       qmcFooterKind
	AudioLen   int
	EKey       string
	RawKey     []byte
	SongID     uint32
	MID        string
	Filename   string
	FooterSize int
}

func parseQMCFooter(data []byte) qmcFooter {
	result := qmcFooter{Kind: qmcFooterUnknown, AudioLen: len(data)}
	n := len(data)
	if n < 8 {
		return result
	}

	if n >= 16 && bytes.Equal(data[n-8:], []byte("musicex\x00")) {
		footerSize := int(binary.LittleEndian.Uint32(data[n-16 : n-12]))
		version := binary.LittleEndian.Uint32(data[n-12 : n-8])
		if version == 1 && footerSize >= 16 && footerSize <= n {
			footerStart := n - footerSize
			meta := data[footerStart : n-16]
			if len(meta) >= 0x48 {
				result.Kind = qmcFooterMusicEx
				result.AudioLen = footerStart
				result.FooterSize = footerSize
				result.SongID = binary.LittleEndian.Uint32(meta[:4])
				result.MID = readUTF16LE(meta, 0x0c, 60)
				result.Filename = readUTF16LE(meta, 0x48, 68)
				return result
			}
		}
	}

	if bytes.Equal(data[n-4:], []byte("QTag")) {
		metaSize := int(binary.BigEndian.Uint32(data[n-8 : n-4]))
		if metaSize > 0 && metaSize <= n-8 {
			metaStart := n - 8 - metaSize
			meta := data[metaStart : n-8]
			parts := bytes.SplitN(meta, []byte{','}, 3)
			if len(parts) >= 2 && len(parts[0]) > 0 {
				result.Kind = qmcFooterQTag
				result.AudioLen = metaStart
				result.FooterSize = metaSize + 8
				result.EKey = string(parts[0])
				return result
			}
		}
	}

	if bytes.Equal(data[n-4:], []byte("STag")) {
		result.Kind = qmcFooterSTag
		return result
	}

	keySize := int(binary.LittleEndian.Uint32(data[n-4:]))
	if keySize > 0 && keySize <= 0x400 && keySize <= n-4 {
		keyStart := n - 4 - keySize
		result.Kind = qmcFooterV1
		result.AudioLen = keyStart
		result.FooterSize = keySize + 4
		result.RawKey = append([]byte(nil), data[keyStart:n-4]...)
	}

	return result
}

const maxQMC2FooterSize = 16 * 1024 * 1024

// readQMCFooterAt reads only the encrypted file's footer. The returned audio
// length is absolute, while qmcFooter keeps the existing in-memory parser's
// metadata representation.
func readQMCFooterAt(reader io.ReaderAt, fileSize int64) (qmcFooter, int64, error) {
	if fileSize <= 0 {
		return qmcFooter{}, 0, errors.New("QQ 音乐文件为空")
	}
	probeSize := int64(16)
	if fileSize < probeSize {
		probeSize = fileSize
	}
	probe := make([]byte, int(probeSize))
	if _, err := reader.ReadAt(probe, fileSize-probeSize); err != nil && err != io.EOF {
		return qmcFooter{}, 0, errors.New("读取 QQ 音乐文件尾部失败")
	}

	footerSize := 0
	probeLen := len(probe)
	if probeLen >= 16 && bytes.Equal(probe[probeLen-8:], []byte("musicex\x00")) {
		footerSize = int(binary.LittleEndian.Uint32(probe[probeLen-16 : probeLen-12]))
		version := binary.LittleEndian.Uint32(probe[probeLen-12 : probeLen-8])
		if version != 1 || footerSize < 16 {
			return qmcFooter{}, 0, errors.New("MusicEx 尾部版本或长度无效")
		}
	} else if probeLen >= 8 && bytes.Equal(probe[probeLen-4:], []byte("QTag")) {
		metadataSize := int(binary.BigEndian.Uint32(probe[probeLen-8 : probeLen-4]))
		if metadataSize <= 0 {
			return qmcFooter{}, 0, errors.New("QTag 尾部长度无效")
		}
		footerSize = metadataSize + 8
	} else if probeLen >= 4 && bytes.Equal(probe[probeLen-4:], []byte("STag")) {
		return qmcFooter{Kind: qmcFooterSTag}, fileSize, nil
	} else if probeLen >= 4 {
		keySize := int(binary.LittleEndian.Uint32(probe[probeLen-4:]))
		if keySize > 0 && keySize <= 0x400 {
			footerSize = keySize + 4
		}
	}

	if footerSize == 0 {
		return qmcFooter{Kind: qmcFooterUnknown}, fileSize, nil
	}
	if footerSize > maxQMC2FooterSize || int64(footerSize) > fileSize {
		return qmcFooter{}, 0, errors.New("QQ 音乐文件尾部长度超出文件范围")
	}
	footerData := make([]byte, footerSize)
	if n, err := reader.ReadAt(footerData, fileSize-int64(footerSize)); n != len(footerData) || (err != nil && err != io.EOF) {
		return qmcFooter{}, 0, errors.New("QQ 音乐文件尾部不完整")
	}
	footer := parseQMCFooter(footerData)
	if footer.Kind == qmcFooterUnknown || footer.FooterSize != footerSize {
		return qmcFooter{}, 0, errors.New("QQ 音乐文件尾部损坏")
	}
	audioLen := fileSize - int64(footerSize)
	if audioLen <= 0 {
		return qmcFooter{}, 0, errors.New("QQ 音乐文件没有音频数据")
	}
	return footer, audioLen, nil
}

func readUTF16LE(data []byte, offset int, maxBytes int) string {
	if offset < 0 || offset >= len(data) || maxBytes <= 0 {
		return ""
	}
	end := offset + maxBytes
	if end > len(data) {
		end = len(data)
	}
	units := make([]uint16, 0, (end-offset)/2)
	for i := offset; i+1 < end; i += 2 {
		unit := binary.LittleEndian.Uint16(data[i : i+2])
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return string(utf16.Decode(units))
}

func resolveQMC2Keys(
	ctx context.Context,
	footer qmcFooter,
	inputName string,
	options DecodeOptions,
	onProgress ProgressCallback,
) ([][]byte, error) {
	explicitEKey := strings.TrimSpace(options.QQEKey)
	if explicitEKey != "" {
		key, err := parseQMC2EKey(explicitEKey)
		if err != nil {
			return nil, fmt.Errorf("QQ 音乐密钥解析失败: %w", err)
		}
		return [][]byte{key}, nil
	}

	var (
		keys [][]byte
		err  error
	)
	switch footer.Kind {
	case qmcFooterQTag:
		var key []byte
		key, err = parseQMC2EKey(footer.EKey)
		if err == nil {
			keys = [][]byte{key}
		}
	case qmcFooterV1:
		keys, err = parseEmbeddedQMC2Keys(footer.RawKey)
	case qmcFooterMusicEx:
		cookie := strings.TrimSpace(options.QQCookie)
		if cookie == "" && !options.QQAutoLogin {
			return nil, fmt.Errorf(
				"%s 使用 MusicEx 新格式，请勾选“自动使用本机 QQ 音乐登录状态”，或在高级选项填写该文件的 EKey/Cookie",
				inputName,
			)
		}
		if footer.MID == "" || footer.Filename == "" {
			return nil, errors.New("MusicEx 尾部缺少歌曲 MID 或内部文件名")
		}
		if onProgress != nil {
			onProgress(8)
		}
		var ekey string
		if cookie != "" {
			ekey, err = fetchQQEKey(ctx, footer.MID, footer.Filename, cookie)
		}
		if (cookie == "" || err != nil) && options.QQAutoLogin {
			ekey, err = fetchQQLocalEKey(ctx, footer.MID, footer.Filename)
		}
		if err == nil {
			var key []byte
			key, err = parseQMC2EKey(ekey)
			if err == nil {
				keys = [][]byte{key}
			}
		}
	case qmcFooterSTag:
		return nil, fmt.Errorf("%s 使用无内嵌密钥的 STag 格式，请为该文件填写 EKey", inputName)
	default:
		return nil, fmt.Errorf("无法识别 %s 的 QMC2 密钥尾部，请为该文件填写 EKey", inputName)
	}
	if err != nil {
		return nil, fmt.Errorf("QQ 音乐密钥解析失败: %w", err)
	}
	if len(keys) == 0 {
		return nil, errors.New("QQ 音乐密钥为空")
	}
	return keys, nil
}

func decryptQMC2Data(
	ctx context.Context,
	data []byte,
	inputName string,
	options DecodeOptions,
	onProgress ProgressCallback,
) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("QQ 音乐文件为空")
	}

	footer := parseQMCFooter(data)
	if footer.AudioLen <= 0 || footer.AudioLen > len(data) {
		return nil, errors.New("QQ 音乐文件尾部损坏")
	}

	keys, err := resolveQMC2Keys(ctx, footer, inputName, options, onProgress)
	if err != nil {
		return nil, err
	}

	const chunkSize = 1024 * 1024
	for _, key := range keys {
		cipher, err := newQMC2Cipher(key)
		if err != nil {
			continue
		}
		decrypted := append([]byte(nil), data[:footer.AudioLen]...)
		for start := 0; start < len(decrypted); start += chunkSize {
			select {
			case <-ctx.Done():
				return nil, errors.New("任务已取消")
			default:
			}
			end := start + chunkSize
			if end > len(decrypted) {
				end = len(decrypted)
			}
			cipher.decrypt(start, decrypted[start:end])
			if onProgress != nil {
				percent := 10 + int(int64(end)*80/int64(len(decrypted)))
				onProgress(percent)
			}
		}
		if hasAudioMagic(decrypted) {
			return decrypted, nil
		}
	}
	return nil, errors.New("解密结果不是可识别音频，EKey/登录凭据可能无效或当前账号无权访问该歌曲")
}

func decryptQMC2File(
	ctx context.Context,
	inputPath string,
	outputPath string,
	options DecodeOptions,
	onProgress ProgressCallback,
) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	footer, audioLen, err := readQMCFooterAt(input, info.Size())
	if err != nil {
		return err
	}
	if uint64(audioLen) > uint64(^uint(0)>>1) {
		return errors.New("QQ 音乐文件过大，当前平台无法处理")
	}
	if onProgress != nil {
		onProgress(5)
	}
	keys, err := resolveQMC2Keys(ctx, footer, filepath.Base(inputPath), options, onProgress)
	if err != nil {
		return err
	}

	probeSize := int64(64)
	if audioLen < probeSize {
		probeSize = audioLen
	}
	probe := make([]byte, int(probeSize))
	if n, readErr := input.ReadAt(probe, 0); n != len(probe) || (readErr != nil && readErr != io.EOF) {
		return errors.New("读取 QQ 音乐音频头失败")
	}
	cipher, err := selectQMC2Cipher(keys, probe)
	if err != nil {
		return err
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		_ = output.Close()
		if !completed {
			_ = os.Remove(outputPath)
		}
	}()

	reader := io.NewSectionReader(input, 0, audioLen)
	buffer := make([]byte, 1024*1024)
	var offset int64
	lastPercent := -1
	for {
		select {
		case <-ctx.Done():
			return errors.New("任务已取消")
		default:
		}

		n, readErr := reader.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			cipher.decrypt(int(offset), chunk)
			if _, err := output.Write(chunk); err != nil {
				return err
			}
			offset += int64(n)
			if onProgress != nil && audioLen > 0 {
				percent := 10 + int(offset*80/audioLen)
				if percent > 90 {
					percent = 90
				}
				if percent != lastPercent {
					lastPercent = percent
					onProgress(percent)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := output.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

func selectQMC2Cipher(keys [][]byte, encryptedProbe []byte) (qmc2StreamCipher, error) {
	for _, key := range keys {
		cipher, err := newQMC2Cipher(key)
		if err != nil {
			continue
		}
		probe := append([]byte(nil), encryptedProbe...)
		cipher.decrypt(0, probe)
		if hasAudioMagic(probe) {
			return cipher, nil
		}
	}
	return nil, errors.New("解密结果不是可识别音频，EKey/登录凭据可能无效或当前账号无权访问该歌曲")
}

func parseEmbeddedQMC2Keys(raw []byte) ([][]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("内嵌密钥为空")
	}
	var candidates [][]byte
	if key, err := deriveQMC2DecodedKey(raw); err == nil {
		candidates = append(candidates, key)
	}
	if key, err := parseQMC2EKey(string(raw)); err == nil {
		duplicate := false
		for _, candidate := range candidates {
			if bytes.Equal(candidate, key) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("无法解析内嵌 V1 密钥")
	}
	return candidates, nil
}

func parseQMC2EKey(ekey string) ([]byte, error) {
	decoded, err := decodeFlexibleBase64(ekey)
	if err != nil {
		return nil, errors.New("EKey 不是有效的 Base64 数据")
	}
	return deriveQMC2DecodedKey(decoded)
}

func decodeFlexibleBase64(value string) ([]byte, error) {
	value = strings.Trim(strings.TrimSpace(value), "\x00")
	value = strings.ReplaceAll(value, " ", "+")
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

var (
	qmc2EncV2Prefix = []byte("QQMusic EncV2,Key:")
	qmc2EncV2Key1   = []byte("386ZJY!@#*$%^&)(")
	qmc2EncV2Key2   = []byte("**#!(#$%&^a1cZ,T")
	qmc2SimpleKey   = [8]byte{0x69, 0x56, 0x46, 0x38, 0x2b, 0x20, 0x15, 0x0b}
)

func deriveQMC2DecodedKey(decoded []byte) ([]byte, error) {
	if bytes.HasPrefix(decoded, qmc2EncV2Prefix) {
		stage1, ok := tcTeaCBCDecrypt(decoded[len(qmc2EncV2Prefix):], qmc2EncV2Key1)
		if !ok {
			return nil, errors.New("EncV2 第一阶段解密失败")
		}
		stage2, ok := tcTeaCBCDecrypt(stage1, qmc2EncV2Key2)
		if !ok {
			return nil, errors.New("EncV2 第二阶段解密失败")
		}
		var err error
		decoded, err = decodeFlexibleBase64(string(stage2))
		if err != nil {
			return nil, errors.New("EncV2 内层 EKey 无效")
		}
	}

	if len(decoded) < 8 {
		return nil, errors.New("EKey 解码后长度不足")
	}
	header := decoded[:8]
	body := decoded[8:]
	if len(body) == 0 {
		return append([]byte(nil), header...), nil
	}

	var teaKey [16]byte
	for i := 0; i < 8; i++ {
		teaKey[i*2] = qmc2SimpleKey[i]
		teaKey[i*2+1] = header[i]
	}
	if decryptedBody, ok := tcTeaCBCDecrypt(body, teaKey[:]); ok {
		key := make([]byte, 0, 8+len(decryptedBody))
		key = append(key, header...)
		key = append(key, decryptedBody...)
		return key, nil
	}

	// EKeys returned by current QQ APIs may already be raw key material.
	return append([]byte(nil), decoded...), nil
}

func tcTeaCBCDecrypt(ciphertext []byte, key []byte) ([]byte, bool) {
	if len(key) != 16 || len(ciphertext) < 16 || len(ciphertext)%8 != 0 {
		return nil, false
	}
	result := append([]byte(nil), ciphertext...)
	var iv1, iv2 [8]byte
	for offset := 0; offset < len(result); offset += 8 {
		block := append([]byte(nil), result[offset:offset+8]...)
		var next [8]byte
		copy(next[:], block)
		for i := 0; i < 8; i++ {
			block[i] ^= iv2[i]
		}

		v0 := binary.BigEndian.Uint32(block[:4])
		v1 := binary.BigEndian.Uint32(block[4:])
		v0, v1 = tcTeaDecryptBlock(v0, v1, key)
		binary.BigEndian.PutUint32(block[:4], v0)
		binary.BigEndian.PutUint32(block[4:], v1)
		copy(iv2[:], block)

		for i := 0; i < 8; i++ {
			block[i] ^= iv1[i]
		}
		iv1 = next
		copy(result[offset:offset+8], block)
	}

	padding := int(result[0]&0x07) + 2
	start := 1 + padding
	end := len(result) - 7
	if end <= start {
		return nil, false
	}
	for _, value := range result[end:] {
		if value != 0 {
			return nil, false
		}
	}
	return append([]byte(nil), result[start:end]...), true
}

func tcTeaDecryptBlock(v0 uint32, v1 uint32, key []byte) (uint32, uint32) {
	k0 := binary.BigEndian.Uint32(key[0:4])
	k1 := binary.BigEndian.Uint32(key[4:8])
	k2 := binary.BigEndian.Uint32(key[8:12])
	k3 := binary.BigEndian.Uint32(key[12:16])
	const delta uint32 = 0x9e3779b9
	sum := uint32(0xe3779b90)
	for i := 0; i < 16; i++ {
		v1 -= ((v0 << 4) + k2) ^ (v0 + sum) ^ ((v0 >> 5) + k3)
		v0 -= ((v1 << 4) + k0) ^ (v1 + sum) ^ ((v1 >> 5) + k1)
		sum -= delta
	}
	return v0, v1
}

type qmc2StreamCipher interface {
	decrypt(offset int, data []byte)
}

func newQMC2Cipher(key []byte) (qmc2StreamCipher, error) {
	if len(key) == 0 {
		return nil, errors.New("QMC2 密钥为空")
	}
	if len(key) > 300 {
		return newQMC2RC4Cipher(key), nil
	}
	return &qmc2MapCipher{key: append([]byte(nil), key...)}, nil
}

type qmc2MapCipher struct {
	key []byte
}

func (cipher *qmc2MapCipher) decrypt(offset int, data []byte) {
	for i := range data {
		current := offset + i
		if current > 0x7fff {
			current %= 0x7fff
		}
		index := (current*current + 71214) % len(cipher.key)
		rotation := (index + 4) & 7
		value := uint16(cipher.key[index])
		mask := byte(((value << rotation) | (value >> rotation)) & 0xff)
		data[i] ^= mask
	}
}

const (
	qmc2FirstSegment = 0x80
	qmc2SegmentSize  = 0x1400
)

type qmc2RC4Cipher struct {
	key  []byte
	sBox []byte
	hash uint32
}

func newQMC2RC4Cipher(key []byte) *qmc2RC4Cipher {
	n := len(key)
	sBox := make([]byte, n)
	for i := range sBox {
		sBox[i] = byte(i)
	}
	j := 0
	for i := 0; i < n; i++ {
		j = (j + int(sBox[i]) + int(key[i%n])) % n
		sBox[i], sBox[j] = sBox[j], sBox[i]
	}

	hash := uint32(1)
	for _, value := range key {
		if value == 0 {
			continue
		}
		next := hash * uint32(value)
		if next == 0 || next <= hash {
			break
		}
		hash = next
	}
	return &qmc2RC4Cipher{
		key:  append([]byte(nil), key...),
		sBox: sBox,
		hash: hash,
	}
}

func (cipher *qmc2RC4Cipher) segmentKey(id int) int {
	seed := cipher.key[id%len(cipher.key)]
	if seed == 0 {
		return 0
	}
	value := float64(cipher.hash) / float64((id+1)*int(seed)) * 100
	return int(value) % len(cipher.key)
}

func (cipher *qmc2RC4Cipher) decrypt(offset int, data []byte) {
	remaining := len(data)
	processed := 0
	if offset < qmc2FirstSegment {
		length := qmc2FirstSegment - offset
		if length > remaining {
			length = remaining
		}
		for i := 0; i < length; i++ {
			data[i] ^= cipher.key[cipher.segmentKey(offset+i)]
		}
		remaining -= length
		processed += length
		offset += length
	}

	if remaining > 0 && offset%qmc2SegmentSize != 0 {
		length := qmc2SegmentSize - offset%qmc2SegmentSize
		if length > remaining {
			length = remaining
		}
		cipher.decryptSegment(data, processed, length, offset)
		remaining -= length
		processed += length
		offset += length
	}

	for remaining > qmc2SegmentSize {
		cipher.decryptSegment(data, processed, qmc2SegmentSize, offset)
		remaining -= qmc2SegmentSize
		processed += qmc2SegmentSize
		offset += qmc2SegmentSize
	}
	if remaining > 0 {
		cipher.decryptSegment(data, processed, remaining, offset)
	}
}

func (cipher *qmc2RC4Cipher) decryptSegment(data []byte, start int, length int, offset int) {
	sBox := append([]byte(nil), cipher.sBox...)
	n := len(sBox)
	skip := offset%qmc2SegmentSize + cipher.segmentKey(offset/qmc2SegmentSize)
	j, k := 0, 0
	for i := -skip; i < length; i++ {
		j = (j + 1) % n
		k = (int(sBox[j]) + k) % n
		sBox[j], sBox[k] = sBox[k], sBox[j]
		if i >= 0 {
			data[start+i] ^= sBox[(int(sBox[j])+int(sBox[k]))%n]
		}
	}
}

func hasAudioMagic(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if bytes.HasPrefix(data, []byte("fLaC")) ||
		bytes.HasPrefix(data, []byte("OggS")) ||
		bytes.HasPrefix(data, []byte("ID3")) ||
		bytes.HasPrefix(data, []byte("MAC ")) {
		return true
	}
	if data[0] == 0xff && data[1]&0xe0 == 0xe0 {
		return true
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return true
	}
	if hasMP4Magic(data) {
		return true
	}
	asf := []byte{
		0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11,
		0xa6, 0xd9, 0x00, 0xaa, 0x00, 0x62, 0xce, 0x6c,
	}
	if len(data) >= len(asf) && bytes.Equal(data[:len(asf)], asf) {
		return true
	}
	return false
}

func hasMP4Magic(data []byte) bool {
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}
	boxSize := int(binary.BigEndian.Uint32(data[:4]))
	if boxSize != 0 && (boxSize < 12 || boxSize > len(data)) {
		return false
	}
	brand := string(data[8:12])
	switch brand {
	case "M4A ", "M4B ", "isom", "iso2", "iso5", "iso6", "mp41", "mp42", "qt  ", "3gp4", "3gp5", "avc1", "dash":
		return true
	default:
		return false
	}
}
