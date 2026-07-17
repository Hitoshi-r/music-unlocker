package decoder

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExtractQQAuthstCandidates(t *testing.T) {
	const (
		jsonToken    = "JsonToken_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		escapedToken = "EscapedToken_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		cookieToken  = "CookieToken_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		wideToken    = "WideToken_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	)
	data := []byte(
		`{"authst":"` + jsonToken + `","short":"ignored"}` + "\x00" +
			`{\"authst\":\"` + escapedToken + `\"}` + "\x00" +
			`authst=` + cookieToken + `; path=/` + "\x00" +
			`authst=too-short;`,
	)
	data = append(data, utf16LEASCII(`{"authst":"`+wideToken+`"}`)...)

	got := extractQQAuthstCandidates(data)
	want := []string{jsonToken, escapedToken, cookieToken, wideToken}
	for _, token := range want {
		if !containsString(got, token) {
			t.Fatalf("expected token of length %d was not extracted", len(token))
		}
	}
	if containsString(got, "too-short") {
		t.Fatal("short placeholder was accepted as authst")
	}
}

func TestExtractQQAuthstCandidatesRejectsGetSongInfoQuery(t *testing.T) {
	data := []byte(
		"authst=0&sin=0&cv=2111&cmd=getsonginfo&qq=1000000000" +
			"&songids=200000000&songtypes=0&ctx=1",
	)
	if got := extractQQAuthstCandidates(data); len(got) != 0 {
		t.Fatalf("getsonginfo query was accepted as authst: %q", got)
	}
}

func TestExtractQQWebKeyCandidates(t *testing.T) {
	const (
		qqKey = "Q_H_L_TestQQMusicKey_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		wxKey = "W_X_TestQQMusicKey_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	)
	data := []byte(
		`qqmusic_key=` + qqKey + `; path=/` + "\x00" +
			`{\"qqmusic_key\": \"` + qqKey + `\"}` + "\x00" +
			`{"qm_keyst":"` + wxKey + `"}`,
	)
	data = append(data, utf16LEASCII(`qm_keyst=`+wxKey+`;`)...)

	got := extractQQWebKeyCandidates(data)
	want := []string{
		"qqmusic_key=" + qqKey,
		"qm_keyst=" + wxKey,
	}
	if len(got) != len(want) {
		t.Fatalf("candidate count = %d, want %d", len(got), len(want))
	}
	for _, candidate := range want {
		if !containsString(got, candidate) {
			t.Fatalf("missing candidate type %q", strings.SplitN(candidate, "=", 2)[0])
		}
	}
}

func TestParseQQMusicUIN(t *testing.T) {
	if got := parseQQMusicUIN([]byte("[User]\r\nUin=123456789\r\n")); got != "123456789" {
		t.Fatalf("parseQQMusicUIN() = %q", got)
	}
	if got := parseQQMusicUIN(utf16LEASCII("[User]\r\nUserUin=987654321\r\n")); got != "987654321" {
		t.Fatalf("parseQQMusicUIN(UTF-16LE) = %q", got)
	}
	if got := parseQQMusicUIN([]byte("Uin=not-a-number")); got != "" {
		t.Fatalf("invalid UIN was accepted: %q", got)
	}
}

func TestLocalCredentialCacheClearsBetweenBatches(t *testing.T) {
	var calls atomic.Int32
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		calls.Add(1)
		return qqLocalCredentials{UIN: "42", AuthstCandidates: []string{"TestAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"}}, nil
	})

	if _, err := cachedLocalQQMusicCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := cachedLocalQQMusicCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider called %d times before cache clear", calls.Load())
	}

	ClearQQMusicLoginCache()
	if _, err := cachedLocalQQMusicCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("provider called %d times after cache clear", calls.Load())
	}
}

func TestLocalCredentialCacheWaitHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		close(started)
		<-release
		return qqLocalCredentials{UIN: "42", AuthstCandidates: []string{"TestAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"}}, nil
	})

	firstDone := make(chan struct{})
	go func() {
		_, _ = cachedLocalQQMusicCredentials(context.Background())
		close(firstDone)
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	go func() {
		_, err := cachedLocalQQMusicCredentials(ctx)
		waiterDone <- err
	}()
	cancel()
	select {
	case err := <-waiterDone:
		if err == nil || !strings.Contains(err.Error(), "任务已取消") {
			t.Fatalf("cancelled waiter error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancelled waiter remained blocked on the credential scan")
	}

	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("credential provider did not finish")
	}
}

func TestAutoLoginDecryptsMusicExWithoutExposingCredential(t *testing.T) {
	const authst = "SensitiveTestAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	key := []byte("12345678")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Cookie"); got != "uin=42" {
			t.Errorf("unexpected Cookie header: %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		comm, _ := body["comm"].(map[string]any)
		if comm["authst"] != authst {
			t.Error("desktop request did not use the local login token")
		}
		writer.Header().Set("Content-Type", "application/json")
		encodedKey := base64.StdEncoding.EncodeToString(key)
		_, _ = writer.Write([]byte(`{"req_1":{"code":0,"data":{"midurlinfo":[{"filename":"test.mflac","ekey":"` + encodedKey + `","result":0}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		return qqLocalCredentials{UIN: "42", AuthstCandidates: []string{authst}}, nil
	})

	plain := append([]byte("fLaC"), bytes.Repeat([]byte{0x2a}, 256)...)
	encrypted := append([]byte(nil), plain...)
	(&qmc2MapCipher{key: key}).decrypt(0, encrypted)
	data := makeMusicExData(encrypted, 1, "song-mid", "test.mflac")
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "test.mflac")
	outputPath := filepath.Join(directory, "test.raw")
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	err := decryptQMC2File(
		context.Background(),
		inputPath,
		outputPath,
		DecodeOptions{QQAutoLogin: true},
		nil,
	)
	if err != nil {
		if strings.Contains(err.Error(), authst) {
			t.Fatal("credential was exposed in an error")
		}
		t.Fatalf("decryptQMC2File() error = %v", err)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("decrypted data does not match plaintext")
	}
}

func TestFetchQQLocalEKeyPrefersWebKey(t *testing.T) {
	const (
		webValue = "Q_H_L_TestLocalWebKey_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		authst   = "TestFallbackAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if got := request.Header.Get("Cookie"); got != "uin=42; qqmusic_key="+webValue {
			t.Errorf("unexpected Cookie header length: %d", len(got))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if _, ok := body["req_0"]; !ok {
			t.Error("web-key request did not use req_0")
		}
		if _, ok := body["req_1"]; ok {
			t.Error("authst fallback was used before the web key")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"req_0":{"code":0,"data":{"midurlinfo":[{"filename":"test.mflac","ekey":"test-ekey","result":0}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		return qqLocalCredentials{
			UIN:              "42",
			WebKeyCandidates: []string{"qqmusic_key=" + webValue},
			AuthstCandidates: []string{authst},
		}, nil
	})

	got, err := fetchQQLocalEKey(context.Background(), "song-mid", "test.mflac")
	if err != nil {
		t.Fatal(err)
	}
	if got != "test-ekey" || calls.Load() != 1 {
		t.Fatalf("unexpected EKey length/call count: %d/%d", len(got), calls.Load())
	}
}

func TestCheckLocalQQMusicLoginReturnsOnlyMaskedStatus(t *testing.T) {
	const authst = "SensitiveLocalStatusAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	key := []byte("12345678")
	encodedKey := base64.StdEncoding.EncodeToString(key)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"req_1":{"code":0,"data":{"midurlinfo":[{"filename":"test.mflac","ekey":"` + encodedKey + `","result":0}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		return qqLocalCredentials{UIN: "1234567890", AuthstCandidates: []string{authst}}, nil
	})

	path := filepath.Join(t.TempDir(), "test.mflac")
	if err := os.WriteFile(path, makeMusicExData([]byte{1, 2, 3, 4}, 1, "song-mid", "test.mflac"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := CheckLocalQQMusicLogin(context.Background(), path)
	if err != nil {
		t.Fatalf("CheckLocalQQMusicLogin() error = %v", err)
	}
	if result.State != "authorized" || result.Account != "123****890" {
		t.Fatalf("unexpected local login result: %#v", result)
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{authst, encodedKey, "1234567890"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("local login result exposed sensitive value of length %d", len(secret))
		}
	}
}

func TestCheckLocalQQMusicLoginSupportsWebKeyWithoutExposingCredential(t *testing.T) {
	const webValue = "Q_H_L_LocalStatusWebKey_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	key := []byte("12345678")
	encodedKey := base64.StdEncoding.EncodeToString(key)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if _, ok := body["req_0"]; !ok {
			t.Error("local web key did not use the web endpoint")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"req_0":{"code":0,"data":{"midurlinfo":[{"filename":"test.mflac","ekey":"` + encodedKey + `","result":0}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)
	replaceLocalQQCredentialProviderForTest(t, func(context.Context) (qqLocalCredentials, error) {
		return qqLocalCredentials{
			UIN:              "1234567890",
			WebKeyCandidates: []string{"qqmusic_key=" + webValue},
		}, nil
	})

	path := filepath.Join(t.TempDir(), "test.mflac")
	if err := os.WriteFile(path, makeMusicExData([]byte{1, 2, 3, 4}, 1, "song-mid", "test.mflac"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := CheckLocalQQMusicLogin(context.Background(), path)
	if err != nil {
		t.Fatalf("CheckLocalQQMusicLogin() error = %v", err)
	}
	if result.State != "authorized" || result.LoginType != "2" || result.Account != "123****890" {
		t.Fatalf("unexpected local login result: %#v", result)
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{webValue, encodedKey, "1234567890"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("local login result exposed sensitive value of length %d", len(secret))
		}
	}
}

func TestOptionalLocalQQMusicLogin(t *testing.T) {
	if os.Getenv("QMC_TEST_LOCAL_QQ_LOGIN") != "1" {
		t.Skip("set QMC_TEST_LOCAL_QQ_LOGIN=1 to test the running QQ Music client")
	}
	credentials, err := getLocalQQMusicCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials.WebKeyCandidates) == 0 && len(credentials.AuthstCandidates) == 0 {
		t.Fatal("no local QQ Music login token was found")
	}
	for _, candidate := range credentials.WebKeyCandidates {
		_, value, found := strings.Cut(candidate, "=")
		if !found || !isPlausibleQQWebKey(value) {
			t.Fatal("local QQ Music web key did not pass validation")
		}
	}
	for _, candidate := range credentials.AuthstCandidates {
		if !isPlausibleQQAuthst(candidate) {
			t.Fatal("local QQ Music login token did not pass validation")
		}
	}
	t.Logf(
		"validated %d web-key candidate(s) and %d desktop-token candidate(s)",
		len(credentials.WebKeyCandidates),
		len(credentials.AuthstCandidates),
	)
}

func replaceLocalQQCredentialProviderForTest(
	t *testing.T,
	provider func(context.Context) (qqLocalCredentials, error),
) {
	t.Helper()
	oldProvider := localQQCredentialProvider
	ClearQQMusicLoginCache()
	localQQCredentialProvider = provider
	t.Cleanup(func() {
		ClearQQMusicLoginCache()
		localQQCredentialProvider = oldProvider
	})
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func utf16LEASCII(value string) []byte {
	result := make([]byte, 0, len(value)*2)
	for index := 0; index < len(value); index++ {
		result = append(result, value[index], 0)
	}
	return result
}
