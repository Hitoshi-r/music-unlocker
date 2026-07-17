package decoder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// QQCookieCheckResult contains only non-sensitive validation details. The
// original Cookie and the fetched EKey are never returned to the frontend.
type QQCookieCheckResult struct {
	State           string `json:"state"`
	Message         string `json:"message"`
	LoginType       string `json:"loginType"`
	Account         string `json:"account"`
	LocalAccount    string `json:"localAccount"`
	AccountMismatch bool   `json:"accountMismatch"`
}

// CheckQQMusicCookie checks the Cookie structure and, when a MusicEx file is
// supplied, asks QQ Music for that file's EKey. It never decrypts or returns
// credentials as part of the check result.
func CheckQQMusicCookie(ctx context.Context, rawCookie string, inputPath string) (QQCookieCheckResult, error) {
	cookie := normalizeQQCookie(rawCookie)
	if cookie == "" {
		return QQCookieCheckResult{}, errors.New("请先填写 QQ Music Cookie")
	}
	if len(cookie) > 32*1024 {
		return QQCookieCheckResult{}, errors.New("QQ Music Cookie 长度异常")
	}
	if strings.ContainsAny(cookie, "\r\n\x00") {
		return QQCookieCheckResult{}, errors.New("QQ Music Cookie 包含无效换行或控制字符")
	}

	key := cookieValue(cookie, "qqmusic_key")
	if key == "" {
		key = cookieValue(cookie, "qm_keyst")
	}
	authst := cookieValue(cookie, "authst")
	if key == "" && authst == "" {
		return QQCookieCheckResult{}, errors.New("Cookie 中缺少 qqmusic_key、qm_keyst 或 authst")
	}

	loginType := "3"
	loginLabel := "QQ 音乐桌面登录"
	if key != "" {
		var err error
		loginType, err = qqCookieLoginType(cookie, key)
		if err != nil {
			return QQCookieCheckResult{}, err
		}
		if loginType == "1" {
			loginLabel = "微信登录"
		} else {
			loginLabel = "QQ 登录"
		}
	}

	uin := qqCookieUINForLogin(cookie, loginType)
	localUIN := localQQMusicUINForCheck()
	result := QQCookieCheckResult{
		State:           "parsed",
		LoginType:       loginType,
		Account:         maskQQAccount(uin),
		LocalAccount:    maskQQAccount(localUIN),
		AccountMismatch: uin != "0" && localUIN != "" && uin != localUIN,
	}
	identity := loginLabel
	if result.Account != "" {
		identity += "，账号 " + result.Account
	}
	if result.AccountMismatch {
		identity += "；与本机 QQ 音乐账号 " + result.LocalAccount + " 不同"
	}

	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		result.Message = "Cookie 已添加并识别为" + identity + "；添加 MusicEx 文件后可继续检测该歌曲权限"
		return result, nil
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return QQCookieCheckResult{}, errors.New("读取待检测文件失败")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return QQCookieCheckResult{}, errors.New("读取待检测文件信息失败")
	}
	footer, _, err := readQMCFooterAt(file, info.Size())
	if err != nil {
		return QQCookieCheckResult{}, fmt.Errorf("检查 MusicEx 文件失败: %w", err)
	}
	if footer.Kind != qmcFooterMusicEx {
		result.Message = "Cookie 已添加并识别为" + identity + "；当前检测文件不是需要在线密钥的 MusicEx 格式"
		return result, nil
	}
	if footer.MID == "" || footer.Filename == "" {
		return QQCookieCheckResult{}, errors.New("MusicEx 尾部缺少歌曲 MID 或内部文件名")
	}

	ekey, err := fetchQQEKey(ctx, footer.MID, footer.Filename, cookie)
	if err != nil {
		var resultErr *qqEKeyResultError
		if errors.As(err, &resultErr) && resultErr.Result == 104003 {
			result.State = "unauthorized"
			result.Message = "Cookie 已添加并识别为" + identity + "，但服务器未授权当前文件（104003）；请核对下载账号、会员/购买状态及该音质的权限"
			return result, nil
		}
		return QQCookieCheckResult{}, err
	}
	if _, err := parseQMC2EKey(ekey); err != nil {
		return QQCookieCheckResult{}, fmt.Errorf("服务器返回的 EKey 无法解析: %w", err)
	}

	result.State = "authorized"
	result.Message = "Cookie 验证成功：" + identity + "，当前文件已获解密授权"
	return result, nil
}

// CheckLocalQQMusicLogin validates the currently running QQ Music desktop
// session while exposing only a masked account identifier and status message.
func CheckLocalQQMusicLogin(ctx context.Context, inputPath string) (QQCookieCheckResult, error) {
	credentials, err := cachedLocalQQMusicCredentials(ctx)
	if err != nil {
		return QQCookieCheckResult{}, fmt.Errorf("无法读取本机 QQ 音乐登录状态: %w", err)
	}
	if len(credentials.WebKeyCandidates) == 0 && len(credentials.AuthstCandidates) == 0 {
		return QQCookieCheckResult{}, errors.New("本机 QQ 音乐登录状态中没有可用令牌")
	}

	account := maskQQAccount(credentials.UIN)
	identity := "本机 QQ 音乐登录状态"
	if account != "" {
		identity += "，账号 " + account
	}
	loginType := "3"
	if len(credentials.WebKeyCandidates) > 0 {
		_, value, found := strings.Cut(credentials.WebKeyCandidates[0], "=")
		if found {
			if detectedType, typeErr := qqCookieLoginType("uin="+credentials.UIN+"; "+credentials.WebKeyCandidates[0], value); typeErr == nil {
				loginType = detectedType
			}
		}
	}
	result := QQCookieCheckResult{
		State:        "parsed",
		LoginType:    loginType,
		Account:      account,
		LocalAccount: account,
	}

	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		result.Message = "已读取" + identity + "；添加 MusicEx 文件后可继续检测该歌曲权限。完整 Cookie 不会显示或保存"
		return result, nil
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return QQCookieCheckResult{}, errors.New("读取待检测文件失败")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return QQCookieCheckResult{}, errors.New("读取待检测文件信息失败")
	}
	footer, _, err := readQMCFooterAt(file, info.Size())
	if err != nil {
		return QQCookieCheckResult{}, fmt.Errorf("检查 MusicEx 文件失败: %w", err)
	}
	if footer.Kind != qmcFooterMusicEx {
		result.Message = "已读取" + identity + "；当前检测文件不是需要在线密钥的 MusicEx 格式"
		return result, nil
	}
	if footer.MID == "" || footer.Filename == "" {
		return QQCookieCheckResult{}, errors.New("MusicEx 尾部缺少歌曲 MID 或内部文件名")
	}

	ekey, err := fetchQQLocalEKey(ctx, footer.MID, footer.Filename)
	if err != nil {
		var resultErr *qqEKeyResultError
		if errors.As(err, &resultErr) && resultErr.Result == 104003 {
			result.State = "unauthorized"
			result.Message = "已读取" + identity + "，但该账号未获当前文件授权（104003）；请登录下载该文件的账号并确认会员及音质权限"
			return result, nil
		}
		result.State = "error"
		result.Message = "已读取" + identity + "，但授权检测失败：" + err.Error()
		return result, nil
	}
	if _, err := parseQMC2EKey(ekey); err != nil {
		result.State = "error"
		result.Message = "本机登录返回的 EKey 无法解析：" + err.Error()
		return result, nil
	}

	result.State = "authorized"
	result.Message = "本机登录验证成功：" + identity + "，当前文件已获解密授权"
	return result, nil
}

func maskQQAccount(uin string) string {
	uin = strings.TrimSpace(uin)
	if uin == "" || uin == "0" {
		return ""
	}
	if len(uin) <= 4 {
		return strings.Repeat("*", len(uin))
	}
	if len(uin) <= 7 {
		return uin[:2] + "***" + uin[len(uin)-2:]
	}
	return uin[:3] + "****" + uin[len(uin)-3:]
}
