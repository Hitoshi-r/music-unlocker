package decoder

import (
	"context"
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"music-unlocker/converter"
)

type NCMDecoder struct{}

func (d *NCMDecoder) Name() string {
	return "网易云音乐"
}

func (d *NCMDecoder) Extensions() []string {
	return []string{".ncm"}
}

var coreKey = []byte{
	0x68, 0x7A, 0x48, 0x52,
	0x41, 0x6D, 0x73, 0x6F,
	0x35, 0x6B, 0x49, 0x6E,
	0x62, 0x61, 0x78, 0x57,
}
var finalPath string

func (d *NCMDecoder) Decode(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	fmt.Println(">>> Decode 已进入")

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, err
	}

	fmt.Println("文件头:", string(header))

	if string(header) != "CTENFDAM" {
		return nil, errors.New("不是有效的 NCM 文件")
	}

	if _, err := file.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}

	var keyLen uint32
	if err := binary.Read(file, binary.LittleEndian, &keyLen); err != nil {
		return nil, err
	}

	keyData := make([]byte, keyLen)
	if _, err := io.ReadFull(file, keyData); err != nil {
		return nil, err
	}

	for i := range keyData {
		keyData[i] ^= 0x64
	}

	decryptedKey, err := aesECBDecrypt(keyData, coreKey)
	if err != nil {
		return nil, err
	}

	decryptedKey, err = pkcs7Unpad(decryptedKey)
	if err != nil {
		return nil, err
	}

	if len(decryptedKey) <= 17 {
		return nil, errors.New("解密 key 长度异常")
	}

	musicKey := decryptedKey[17:]

	keyBox := make([]byte, 256)
	for i := 0; i < 256; i++ {
		keyBox[i] = byte(i)
	}

	var j byte
	for i := 0; i < 256; i++ {
		j = j + keyBox[i] + musicKey[i%len(musicKey)]
		keyBox[i], keyBox[j] = keyBox[j], keyBox[i]
	}

	var metaLen uint32
	if err := binary.Read(file, binary.LittleEndian, &metaLen); err != nil {
		return nil, err
	}

	if _, err := file.Seek(int64(metaLen), io.SeekCurrent); err != nil {
		return nil, err
	}

	if _, err := file.Seek(9, io.SeekCurrent); err != nil {
		return nil, err
	}

	var imgLen uint32
	if err := binary.Read(file, binary.LittleEndian, &imgLen); err != nil {
		return nil, err
	}

	if _, err := file.Seek(int64(imgLen), io.SeekCurrent); err != nil {
		return nil, err
	}

	audioStart, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}

	if outputFormat == "" {
		outputFormat = "mp3"
	}

	outputFormat = strings.ToLower(outputFormat)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	rawPath := filepath.Join(outputDir, name+"_raw.bin")

	outFile, err := os.Create(rawPath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		outFile.Close()
		return nil, err
	}

	totalSize := fileInfo.Size() - audioStart
	var processed int64
	lastPercent := -1

	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			outFile.Close()
			os.Remove(rawPath)
			return nil, errors.New("任务已取消")
		default:
		}

		n, readErr := file.Read(buffer)

		if n > 0 {
			data := buffer[:n]

			for i := 0; i < n; i++ {
				globalIndex := processed + int64(i)
				idx := byte((globalIndex + 1) & 0xff)
				t := keyBox[idx]
				k := keyBox[(keyBox[idx]+keyBox[(idx+t)&0xff])&0xff]
				data[i] ^= k
			}

			if _, err := outFile.Write(data); err != nil {
				outFile.Close()
				return nil, err
			}

			processed += int64(n)

			if totalSize > 0 && onProgress != nil {
				percent := int(processed * 100 / totalSize)
				if percent > 100 {
					percent = 100
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
			outFile.Close()
			return nil, readErr
		}
	}

	if err := outFile.Close(); err != nil {
		return nil, err
	}

	fmt.Println("原始解密文件:", rawPath)

	// 自动识别解密后的真实音频格式
	realExt, err := detectAudioExt(rawPath)
	if err != nil {
		return nil, err
	}

	fmt.Println("真实音频格式:", realExt)

	// 如果用户选择“原格式”，或者解密出来的格式和用户选择一致，就直接改名，不转码
	if outputFormat == "" || outputFormat == "origin" || "."+outputFormat == realExt {
		finalPath = filepath.Join(outputDir, name+realExt)

		if err := os.Rename(rawPath, finalPath); err != nil {
			return nil, err
		}

		fmt.Println("无损直出:", finalPath)
	} else {
		finalPath = filepath.Join(outputDir, name+"."+outputFormat)

		fmt.Println("最终输出文件:", finalPath)

		if err := converter.ConvertAudio(rawPath, finalPath, outputFormat, bitrate); err != nil {
			fmt.Println("FFmpeg 转码失败:", err)
			return nil, err
		}

		os.Remove(rawPath)
	}

	if onProgress != nil {
		onProgress(100)
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "NCM 解密完成",
	}, nil
}

// AES ECB 解密
func aesECBDecrypt(cipherText []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(cipherText)%block.BlockSize() != 0 {
		return nil, errors.New("invalid AES block size")
	}

	plainText := make([]byte, len(cipherText))

	for start := 0; start < len(cipherText); start += block.BlockSize() {
		end := start + block.BlockSize()
		block.Decrypt(plainText[start:end], cipherText[start:end])
	}

	return plainText, nil
}

// PKCS7 去填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, errors.New("invalid padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}

	return data[:len(data)-padding], nil
}
func detectAudioExt(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 4)
	n, err := file.Read(header)
	if err != nil {
		return "", err
	}

	if n >= 4 && string(header[:4]) == "fLaC" {
		return ".flac", nil
	}

	if n >= 3 && string(header[:3]) == "ID3" {
		return ".mp3", nil
	}

	if n >= 2 && header[0] == 0xFF && (header[1]&0xE0) == 0xE0 {
		return ".mp3", nil
	}

	return ".bin", nil
}
