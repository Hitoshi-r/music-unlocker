package decoder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	qqMusicAPIURL     = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	qqMusicHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

type qqAPIResponse struct {
	Req0 *qqAPIRequestResponse `json:"req_0"`
	Req1 *qqAPIRequestResponse `json:"req_1"`
}

type qqAPIRequestResponse struct {
	Code int       `json:"code"`
	Data qqAPIData `json:"data"`
}

type qqAPIData struct {
	MidURLInfo []qqMidURLInfo `json:"midurlinfo"`
	Message    string         `json:"msg"`
}

type qqMidURLInfo struct {
	EKey     string `json:"ekey"`
	Filename string `json:"filename"`
	Result   int    `json:"result"`
}

type qqEKeyResultError struct {
	Result int
}

func (err *qqEKeyResultError) Error() string {
	if err.Result == 104003 {
		return "QQ 音乐服务器未为当前文件返回 EKey（result=104003）：可能是登录类型或账号不匹配、下载账号不同、会员/购买或高阶音质权限失效，或试听/版权限制"
	}
	return fmt.Sprintf("QQ 音乐未授权该歌曲（result=%d）", err.Result)
}

func fetchQQEKey(ctx context.Context, songMID string, filename string, cookie string) (string, error) {
	cookie = normalizeQQCookie(cookie)
	if cookie == "" {
		return "", errors.New("QQ Music Cookie 为空")
	}
	if len(cookie) > 32*1024 {
		return "", errors.New("QQ Music Cookie 长度异常")
	}
	if strings.ContainsAny(cookie, "\r\n\x00") {
		return "", errors.New("QQ Music Cookie 包含无效换行或控制字符")
	}

	ekey, webErr := fetchQQWebEKey(ctx, songMID, filename, cookie)
	if webErr == nil && ekey != "" {
		return ekey, nil
	}

	// A full desktop login cookie may expose authst. In that case use the
	// current desktop GetEVkey endpoint as a compatibility fallback.
	authst := cookieValue(cookie, "authst")
	if authst != "" {
		if ekey, desktopErr := fetchQQDesktopEKey(ctx, songMID, filename, cookie, authst); desktopErr == nil {
			return ekey, nil
		} else {
			return "", desktopErr
		}
	}
	if webErr != nil {
		return "", webErr
	}
	return "", errors.New("QQ 音乐接口没有返回 EKey，账号可能无权限或登录已过期")
}

func fetchQQWebEKey(ctx context.Context, songMID string, filename string, cookie string) (string, error) {
	key := cookieValue(cookie, "qqmusic_key")
	if key == "" {
		key = cookieValue(cookie, "qm_keyst")
	}
	if key == "" {
		return "", errors.New("Cookie 中缺少 qqmusic_key 或 qm_keyst")
	}
	loginType, err := qqCookieLoginType(cookie, key)
	if err != nil {
		return "", err
	}
	uin := qqCookieUINForLogin(cookie, loginType)
	token := qqGTK(key)

	body := map[string]any{
		"req_0": map[string]any{
			"module": "music.vkey.GetVkey",
			"method": "UrlGetVkey",
			"param": map[string]any{
				"guid":     "e1b475ed2a9f206084deaf3541d77343590a6454",
				"songmid":  []string{songMID},
				"filename": []string{filename},
				"songtype": []int{0},
				"uin":      uin,
				"ctx":      0,
			},
		},
		"comm": map[string]any{
			"uin":               uin,
			"format":            "json",
			"ct":                24,
			"cv":                4747474,
			"platform":          "yqq.json",
			"chid":              "0",
			"g_tk":              token,
			"g_tk_new_20200303": token,
			"tmeLoginType":      loginType,
			"inCharset":         "utf-8",
			"outCharset":        "utf-8",
			"notice":            0,
			"needNewCode":       1,
		},
	}
	return requestQQEKey(ctx, cookie, body, "req_0")
}

func fetchQQDesktopEKey(
	ctx context.Context,
	songMID string,
	filename string,
	cookie string,
	authst string,
) (string, error) {
	uin := qqCookieUIN(cookie)
	body := map[string]any{
		"comm": map[string]any{
			"authst":       authst,
			"ct":           "19",
			"cv":           "1859",
			"uin":          uin,
			"tmeLoginType": "3",
		},
		"req_1": map[string]any{
			"module": "music.vkey.GetEVkey",
			"method": "CgiGetEVkey",
			"param": map[string]any{
				"filename":  []string{filename},
				"guid":      "10000",
				"songmid":   []string{songMID},
				"songtype":  []int{1},
				"uin":       uin,
				"loginflag": 1,
				"platform":  "27",
				"ctx":       1,
			},
		},
	}
	return requestQQEKey(ctx, cookie, body, "req_1")
}

func requestQQEKey(
	ctx context.Context,
	cookie string,
	body map[string]any,
	requestField string,
) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", errors.New("构造 QQ 音乐请求失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, qqMusicAPIURL, bytes.NewReader(payload))
	if err != nil {
		return "", errors.New("构造 QQ 音乐请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://y.qq.com")
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/125 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := qqMusicHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.New("任务已取消")
		}
		return "", errors.New("连接 QQ 音乐接口失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("QQ 音乐接口返回 HTTP %d", resp.StatusCode)
	}

	responseData, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", errors.New("读取 QQ 音乐接口响应失败")
	}
	var parsed qqAPIResponse
	if err := json.Unmarshal(responseData, &parsed); err != nil {
		return "", errors.New("QQ 音乐接口响应格式异常")
	}

	var result *qqAPIRequestResponse
	if requestField == "req_1" {
		result = parsed.Req1
	} else {
		result = parsed.Req0
	}
	if result == nil {
		return "", errors.New("QQ 音乐接口响应缺少密钥数据")
	}
	if result.Code != 0 {
		return "", fmt.Errorf("QQ 音乐接口请求失败（code=%d）", result.Code)
	}
	if len(result.Data.MidURLInfo) == 0 {
		return "", errors.New("QQ 音乐接口没有返回歌曲密钥")
	}
	info := result.Data.MidURLInfo[0]
	if info.Result != 0 {
		return "", &qqEKeyResultError{Result: info.Result}
	}
	if strings.TrimSpace(info.EKey) == "" {
		return "", errors.New("QQ 音乐接口返回了空 EKey，登录可能已过期或账号无歌曲权限")
	}
	return strings.TrimSpace(info.EKey), nil
}

func normalizeQQCookie(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(cookie), "cookie:") {
		cookie = strings.TrimSpace(cookie[len("cookie:"):])
	}
	lower := strings.ToLower(cookie)
	if strings.Contains(cookie, ";") ||
		strings.HasPrefix(lower, "qqmusic_key=") ||
		strings.HasPrefix(lower, "qm_keyst=") ||
		strings.HasPrefix(lower, "authst=") {
		return cookie
	}
	return "qqmusic_key=" + cookie
}

func qqCookieLoginType(cookie string, key string) (string, error) {
	explicit := strings.TrimSpace(cookieValue(cookie, "tmeLoginType"))
	if explicit != "" && explicit != "1" && explicit != "2" {
		return "", errors.New("Cookie 中的 tmeLoginType 无效")
	}

	inferred := ""
	upperKey := strings.ToUpper(strings.TrimSpace(key))
	switch {
	case strings.HasPrefix(upperKey, "W_X_"):
		inferred = "1"
	case strings.HasPrefix(upperKey, "Q_H_L_"):
		inferred = "2"
	}
	if inferred == "" {
		switch strings.TrimSpace(cookieValue(cookie, "login_type")) {
		case "1":
			inferred = "2"
		case "2":
			inferred = "1"
		}
	}
	if inferred == "" {
		if numericQQCookieValue(cookie, "wxuin") != "" {
			inferred = "1"
		} else if numericQQCookieValue(cookie, "uin") != "" {
			inferred = "2"
		}
	}

	if explicit != "" {
		if inferred != "" && explicit != inferred {
			return "", errors.New("Cookie 的登录类型与密钥类型冲突，请重新复制当前账号的完整 Cookie")
		}
		return explicit, nil
	}
	if inferred != "" {
		return inferred, nil
	}
	// Raw qqmusic_key values are normally QQ-login keys. A full Cookie can
	// override this through tmeLoginType or login_type.
	return "2", nil
}

func cookieValue(cookie string, name string) string {
	for _, part := range strings.Split(cookie, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if found && strings.EqualFold(strings.TrimSpace(key), name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func qqCookieUIN(cookie string) string {
	key := cookieValue(cookie, "qqmusic_key")
	if key == "" {
		key = cookieValue(cookie, "qm_keyst")
	}
	loginType, err := qqCookieLoginType(cookie, key)
	if err != nil {
		return "0"
	}
	return qqCookieUINForLogin(cookie, loginType)
}

func qqCookieUINForLogin(cookie string, loginType string) string {
	names := []string{"uin"}
	if loginType == "1" {
		names = []string{"wxuin", "uin"}
	}
	for _, name := range names {
		if value := numericQQCookieValue(cookie, name); value != "" {
			return value
		}
	}
	return "0"
}

func numericQQCookieValue(cookie string, name string) string {
	value := strings.TrimSpace(cookieValue(cookie, name))
	if len(value) > 1 && (value[0] == 'o' || value[0] == 'O') {
		value = value[1:]
	}
	if value == "" {
		return ""
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return ""
		}
	}
	return value
}

func qqGTK(key string) uint32 {
	hash := uint32(5381)
	for _, char := range key {
		hash += (hash << 5) + uint32(char)
	}
	return hash & 0x7fffffff
}
