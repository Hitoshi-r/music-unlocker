//go:build windows

package decoder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	qqMemoryChunkSize    = 1024 * 1024
	qqMemoryScanLimit    = int64(1024 * 1024 * 1024)
	qqProcessWebKeyLimit = 8
	qqProcessAuthstLimit = 8
	qqWebKeyLimit        = 16
	qqAuthstLimit        = 16
)

func getLocalQQMusicCredentials(ctx context.Context) (qqLocalCredentials, error) {
	credentials := qqLocalCredentials{UIN: readLocalQQMusicUIN()}
	seenWebKeys := make(map[string]struct{})
	seenAuthsts := make(map[string]struct{})
	addWebKeys := func(candidates []string, limit int) {
		for _, candidate := range candidates {
			if len(credentials.WebKeyCandidates) >= limit {
				return
			}
			if _, exists := seenWebKeys[candidate]; exists {
				continue
			}
			seenWebKeys[candidate] = struct{}{}
			credentials.WebKeyCandidates = append(credentials.WebKeyCandidates, candidate)
		}
	}
	addAuthsts := func(candidates []string, limit int) {
		for _, candidate := range candidates {
			if len(credentials.AuthstCandidates) >= limit {
				return
			}
			if _, exists := seenAuthsts[candidate]; exists {
				continue
			}
			seenAuthsts[candidate] = struct{}{}
			credentials.AuthstCandidates = append(credentials.AuthstCandidates, candidate)
		}
	}

	processIDs, err := qqMusicProcessIDs()
	if err != nil {
		return qqLocalCredentials{}, errors.New("无法枚举本机进程")
	}
	var accessErr error
	for _, processID := range processIDs {
		select {
		case <-ctx.Done():
			return qqLocalCredentials{}, errors.New("任务已取消")
		default:
		}
		webKeys, authsts, scanErr := scanQQMusicProcess(ctx, processID)
		if scanErr != nil {
			accessErr = scanErr
			continue
		}
		addWebKeys(webKeys, qqProcessWebKeyLimit)
		addAuthsts(authsts, qqProcessAuthstLimit)
		if len(credentials.WebKeyCandidates) >= qqProcessWebKeyLimit &&
			len(credentials.AuthstCandidates) >= qqProcessAuthstLimit {
			break
		}
	}

	// A few QQ Music releases also leave the login snapshot in a local data
	// file. This is a read-only fallback; the files are never modified.
	// Always include these candidates: process memory can contain stale login
	// snapshots even when a newer file-backed token is available.
	for _, path := range localQQCredentialFiles() {
		webKeys, authsts := readQQLocalCredentialFile(path)
		addWebKeys(webKeys, qqWebKeyLimit)
		addAuthsts(authsts, qqAuthstLimit)
	}

	if len(credentials.WebKeyCandidates) > 0 || len(credentials.AuthstCandidates) > 0 {
		return credentials, nil
	}
	if len(processIDs) == 0 {
		return qqLocalCredentials{}, errors.New("未检测到正在运行的 QQ 音乐；请先打开 QQ 音乐客户端并完成登录")
	}
	if accessErr != nil {
		if errors.Is(accessErr, windows.ERROR_ACCESS_DENIED) {
			return qqLocalCredentials{}, errors.New("无法读取 QQMusic.exe；请让本程序与 QQ 音乐使用相同的普通用户权限运行")
		}
		return qqLocalCredentials{}, fmt.Errorf("读取 QQMusic.exe 失败: %w", accessErr)
	}
	return qqLocalCredentials{}, errors.New("已检测到 QQ 音乐，但没有找到可用登录状态；请在 QQ 音乐内重新登录后再试")
}

func qqMusicProcessIDs() ([]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}

	var processIDs []uint32
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "QQMusic.exe") {
			processIDs = append(processIDs, entry.ProcessID)
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return processIDs, nil
}

func scanQQMusicProcess(ctx context.Context, processID uint32) ([]string, []string, error) {
	process, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		processID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer windows.CloseHandle(process)

	buffer := make([]byte, qqMemoryChunkSize)
	defer clearBytes(buffer)
	const overlapSize = maxQQAuthstLength*2 + 256
	var (
		address     uintptr
		total       int64
		tail        []byte
		webKeys     []string
		authsts     []string
		seenWebKeys = make(map[string]struct{})
		seenAuthsts = make(map[string]struct{})
	)
	defer func() { clearBytes(tail) }()

	for total < qqMemoryScanLimit &&
		(len(webKeys) < qqProcessWebKeyLimit || len(authsts) < qqProcessAuthstLimit) {
		select {
		case <-ctx.Done():
			return nil, nil, errors.New("任务已取消")
		default:
		}

		var memory windows.MemoryBasicInformation
		if err := windows.VirtualQueryEx(process, address, &memory, unsafe.Sizeof(memory)); err != nil {
			break
		}
		nextAddress := memory.BaseAddress + memory.RegionSize
		if nextAddress <= address {
			break
		}

		if isQQMemoryReadable(memory) {
			regionRemaining := memory.RegionSize
			regionAddress := memory.BaseAddress
			for regionRemaining > 0 && total < qqMemoryScanLimit &&
				(len(webKeys) < qqProcessWebKeyLimit || len(authsts) < qqProcessAuthstLimit) {
				select {
				case <-ctx.Done():
					return nil, nil, errors.New("任务已取消")
				default:
				}
				readSize := uintptr(len(buffer))
				if regionRemaining < readSize {
					readSize = regionRemaining
				}
				if remainingLimit := qqMemoryScanLimit - total; int64(readSize) > remainingLimit {
					readSize = uintptr(remainingLimit)
				}

				var bytesRead uintptr
				readErr := windows.ReadProcessMemory(process, regionAddress, &buffer[0], readSize, &bytesRead)
				if bytesRead > 0 {
					combined := make([]byte, len(tail)+int(bytesRead))
					copy(combined, tail)
					copy(combined[len(tail):], buffer[:bytesRead])
					for _, candidate := range extractQQWebKeyCandidates(combined) {
						if len(webKeys) >= qqProcessWebKeyLimit {
							break
						}
						if _, exists := seenWebKeys[candidate]; exists {
							continue
						}
						seenWebKeys[candidate] = struct{}{}
						webKeys = append(webKeys, candidate)
					}
					for _, candidate := range extractQQAuthstCandidates(combined) {
						if len(authsts) >= qqProcessAuthstLimit {
							break
						}
						if _, exists := seenAuthsts[candidate]; exists {
							continue
						}
						seenAuthsts[candidate] = struct{}{}
						authsts = append(authsts, candidate)
					}
					clearBytes(tail)
					tailStart := len(combined) - overlapSize
					if tailStart < 0 {
						tailStart = 0
					}
					tail = append(tail[:0], combined[tailStart:]...)
					clearBytes(combined)
				}
				total += int64(readSize)
				regionAddress += readSize
				regionRemaining -= readSize
				if readErr != nil && bytesRead == 0 {
					break
				}
			}
		}
		address = nextAddress
	}
	return webKeys, authsts, nil
}

func isQQMemoryReadable(memory windows.MemoryBasicInformation) bool {
	if memory.State != windows.MEM_COMMIT || memory.Protect&windows.PAGE_GUARD != 0 || memory.Protect&windows.PAGE_NOACCESS != 0 {
		return false
	}
	protection := memory.Protect & 0xff
	return protection == windows.PAGE_READWRITE ||
		protection == windows.PAGE_WRITECOPY ||
		protection == windows.PAGE_EXECUTE_READWRITE ||
		protection == windows.PAGE_EXECUTE_WRITECOPY
}

func readLocalQQMusicUIN() string {
	base := localQQMusicDataDir()
	if base == "" {
		return ""
	}
	for _, name := range []string{"QQMusicServiceConfig.ini", "QQMusicConfig.ini"} {
		data, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			continue
		}
		uin := parseQQMusicUIN(data)
		clearBytes(data)
		if uin != "" {
			return uin
		}
	}
	return ""
}

func localQQMusicUINForCheck() string {
	return readLocalQQMusicUIN()
}

func localQQCredentialFiles() []string {
	base := localQQMusicDataDir()
	if base == "" {
		return nil
	}
	return []string{
		filepath.Join(base, "SetCookie.dat"),
		filepath.Join(base, "_SetCookie.dat"),
		filepath.Join(base, "mmkv", "mmkv.default"),
	}
}

func localQQMusicDataDir() string {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Tencent", "QQMusic")
}

func readQQLocalCredentialFile(path string) ([]string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32*1024*1024))
	if err != nil {
		clearBytes(data)
		return nil, nil
	}
	webKeys := extractQQWebKeyCandidates(data)
	authsts := extractQQAuthstCandidates(data)
	clearBytes(data)
	return webKeys, authsts
}

func clearBytes(data []byte) {
	for index := range data {
		data[index] = 0
	}
}
