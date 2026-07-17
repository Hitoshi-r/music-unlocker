package decoder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const maxQQAuthstLength = 8 * 1024

type qqLocalCredentials struct {
	UIN              string
	AuthstCandidates []string
}

var localQQCredentialProvider = getLocalQQMusicCredentials

var localQQCredentialCache struct {
	mu          sync.Mutex
	loaded      bool
	loading     bool
	ready       chan struct{}
	generation  uint64
	credentials qqLocalCredentials
	err         error
}

func fetchQQLocalEKey(ctx context.Context, songMID string, filename string) (string, error) {
	credentials, err := cachedLocalQQMusicCredentials(ctx)
	if err != nil {
		return "", fmt.Errorf("无法使用本机 QQ 音乐登录状态: %w", err)
	}
	if len(credentials.AuthstCandidates) == 0 {
		return "", errors.New("本机 QQ 音乐登录状态中没有可用令牌")
	}

	uin := strings.TrimSpace(credentials.UIN)
	if uin == "" {
		uin = "0"
	}
	cookie := "uin=" + uin
	var lastErr error
	for _, authst := range credentials.AuthstCandidates {
		select {
		case <-ctx.Done():
			return "", errors.New("任务已取消")
		default:
		}
		ekey, requestErr := fetchQQDesktopEKey(ctx, songMID, filename, cookie, authst)
		if requestErr == nil && strings.TrimSpace(ekey) != "" {
			return ekey, nil
		}
		lastErr = requestErr
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("本机 QQ 音乐登录状态无法取得该歌曲的 EKey")
}

func cachedLocalQQMusicCredentials(ctx context.Context) (qqLocalCredentials, error) {
	for {
		select {
		case <-ctx.Done():
			return qqLocalCredentials{}, errors.New("任务已取消")
		default:
		}

		localQQCredentialCache.mu.Lock()
		if localQQCredentialCache.loaded {
			credentials := cloneQQLocalCredentials(localQQCredentialCache.credentials)
			err := localQQCredentialCache.err
			localQQCredentialCache.mu.Unlock()
			return credentials, err
		}
		if localQQCredentialCache.loading {
			ready := localQQCredentialCache.ready
			localQQCredentialCache.mu.Unlock()
			select {
			case <-ctx.Done():
				return qqLocalCredentials{}, errors.New("任务已取消")
			case <-ready:
				continue
			}
		}

		localQQCredentialCache.loading = true
		localQQCredentialCache.ready = make(chan struct{})
		generation := localQQCredentialCache.generation
		localQQCredentialCache.mu.Unlock()

		credentials, err := localQQCredentialProvider(ctx)
		cancelled := ctx.Err() != nil

		localQQCredentialCache.mu.Lock()
		if !cancelled && generation == localQQCredentialCache.generation {
			localQQCredentialCache.loaded = true
			localQQCredentialCache.credentials = cloneQQLocalCredentials(credentials)
			localQQCredentialCache.err = err
		}
		localQQCredentialCache.loading = false
		close(localQQCredentialCache.ready)
		localQQCredentialCache.ready = nil
		localQQCredentialCache.mu.Unlock()

		if cancelled {
			return qqLocalCredentials{}, errors.New("任务已取消")
		}
		return cloneQQLocalCredentials(credentials), err
	}
}

// ClearQQMusicLoginCache drops the short-lived in-memory login snapshot used
// by one conversion batch. Credentials are never written to disk or returned
// to the frontend.
func ClearQQMusicLoginCache() {
	localQQCredentialCache.mu.Lock()
	defer localQQCredentialCache.mu.Unlock()
	localQQCredentialCache.generation++
	localQQCredentialCache.loaded = false
	localQQCredentialCache.credentials = qqLocalCredentials{}
	localQQCredentialCache.err = nil
}

func cloneQQLocalCredentials(credentials qqLocalCredentials) qqLocalCredentials {
	return qqLocalCredentials{
		UIN:              credentials.UIN,
		AuthstCandidates: append([]string(nil), credentials.AuthstCandidates...),
	}
}

type qqAuthstMarker struct {
	value   []byte
	quoted  bool
	escaped bool
}

func extractQQAuthstCandidates(data []byte) []string {
	markers := []qqAuthstMarker{
		{value: []byte(`"authst":"`), quoted: true},
		{value: []byte(`"authst": "`), quoted: true},
		{value: []byte(`\"authst\":\"`), quoted: true, escaped: true},
		{value: []byte("authst=")},
		{value: []byte("Authst=")},
	}

	seen := make(map[string]struct{})
	results := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if !isPlausibleQQAuthst(value) {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		results = append(results, value)
	}

	for _, marker := range markers {
		for _, value := range extractMarkedASCIIValues(data, marker) {
			add(value)
		}
		if !marker.escaped {
			for _, value := range extractMarkedUTF16LEValues(data, marker) {
				add(value)
			}
		}
	}

	// Real authst values are normally much longer than placeholder strings
	// embedded in application code. Try longer candidates first.
	sort.SliceStable(results, func(i, j int) bool {
		return len(results[i]) > len(results[j])
	})
	return results
}

func extractMarkedASCIIValues(data []byte, marker qqAuthstMarker) []string {
	var results []string
	for offset := 0; offset < len(data); {
		index := bytes.Index(data[offset:], marker.value)
		if index < 0 {
			break
		}
		start := offset + index + len(marker.value)
		end := start
		for end < len(data) && end-start <= maxQQAuthstLength {
			char := data[end]
			if marker.quoted {
				if char == '"' {
					break
				}
			} else if isQQAuthstTerminator(char) {
				break
			}
			end++
		}
		if end-start <= maxQQAuthstLength {
			raw := data[start:end]
			if marker.escaped && len(raw) > 0 && raw[len(raw)-1] == '\\' {
				raw = raw[:len(raw)-1]
			}
			results = append(results, string(raw))
		}
		offset = start
	}
	return results
}

func extractMarkedUTF16LEValues(data []byte, marker qqAuthstMarker) []string {
	wideMarker := make([]byte, 0, len(marker.value)*2)
	for _, char := range marker.value {
		wideMarker = append(wideMarker, char, 0)
	}

	var results []string
	for offset := 0; offset < len(data); {
		index := bytes.Index(data[offset:], wideMarker)
		if index < 0 {
			break
		}
		start := offset + index + len(wideMarker)
		value := make([]byte, 0, 128)
		valid := true
		for pos := start; pos+1 < len(data) && len(value) <= maxQQAuthstLength; pos += 2 {
			char, high := data[pos], data[pos+1]
			if high != 0 {
				valid = false
				break
			}
			if (marker.quoted && char == '"') || (!marker.quoted && isQQAuthstTerminator(char)) {
				break
			}
			value = append(value, char)
		}
		if valid && len(value) <= maxQQAuthstLength {
			results = append(results, string(value))
		}
		offset = start
	}
	return results
}

func isQQAuthstTerminator(char byte) bool {
	return char == 0 || char == ';' || char == '"' || char == '\'' ||
		char == ' ' || char == '\t' || char == '\r' || char == '\n'
}

func isPlausibleQQAuthst(value string) bool {
	if len(value) < 16 || len(value) > maxQQAuthstLength {
		return false
	}
	alphanumeric := 0
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char <= 0x20 || char >= 0x7f || char == '"' || char == '\'' || char == ';' || char == '\\' {
			return false
		}
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			alphanumeric++
		}
	}
	return alphanumeric >= 12
}

func parseQQMusicUIN(data []byte) string {
	// Removing NUL bytes also handles UTF-16LE INI files whose keys and values
	// are ASCII.
	text := strings.ReplaceAll(string(data), "\x00", "")
	text = strings.TrimPrefix(text, "\ufeff")
	for _, line := range strings.FieldsFunc(text, func(char rune) bool {
		return char == '\r' || char == '\n'
	}) {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || (!strings.EqualFold(strings.TrimSpace(key), "uin") && !strings.EqualFold(strings.TrimSpace(key), "useruin")) {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		valid := true
		for _, char := range value {
			if char < '0' || char > '9' {
				valid = false
				break
			}
		}
		if valid {
			return value
		}
	}
	return ""
}
