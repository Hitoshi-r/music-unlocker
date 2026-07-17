package decoder

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNormalizeQQCookie(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw token", input: "abc123", want: "qqmusic_key=abc123"},
		{name: "raw padded token", input: "abc123==", want: "qqmusic_key=abc123=="},
		{name: "named token", input: "qqmusic_key=abc123==", want: "qqmusic_key=abc123=="},
		{name: "full cookie", input: "uin=42; qqmusic_key=abc", want: "uin=42; qqmusic_key=abc"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeQQCookie(test.input); got != test.want {
				t.Fatalf("normalizeQQCookie() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCookieHelpers(t *testing.T) {
	cookie := "uin=o12345; qqmusic_key=secret; other=value"
	if got := cookieValue(cookie, "qqmusic_key"); got != "secret" {
		t.Fatalf("cookieValue() = %q", got)
	}
	if got := qqCookieUIN(cookie); got != "12345" {
		t.Fatalf("qqCookieUIN() = %q", got)
	}
	if got := qqGTK("secret"); got == 0 {
		t.Fatal("qqGTK() returned zero")
	}
}

func TestFetchQQWebEKeyRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Cookie"); got != "uin=42; qqmusic_key=secret" {
			t.Errorf("Cookie header = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		req0, _ := body["req_0"].(map[string]any)
		if req0["module"] != "music.vkey.GetVkey" || req0["method"] != "UrlGetVkey" {
			t.Errorf("unexpected request body: %#v", req0)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"req_0":{"code":0,"data":{"midurlinfo":[{"filename":"test.mgg","ekey":"test-ekey","result":0}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)

	got, err := fetchQQWebEKey(context.Background(), "song-mid", "test.mgg", "uin=42; qqmusic_key=secret")
	if err != nil {
		t.Fatalf("fetchQQWebEKey() error = %v", err)
	}
	if got != "test-ekey" {
		t.Fatalf("EKey = %q", got)
	}
}

func TestAuthstFallbackReturnsDesktopAuthorizationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"req_1":{"code":0,"data":{"midurlinfo":[{"filename":"test.mgg","ekey":"","result":104003}]}}}`))
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)

	_, err := fetchQQEKey(
		context.Background(),
		"song-mid",
		"test.mgg",
		"uin=42; authst=TestAuthst_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	)
	if err == nil {
		t.Fatal("fetchQQEKey() unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "104003") {
		t.Fatalf("desktop authorization error was hidden: %v", err)
	}
	if strings.Contains(err.Error(), "qqmusic_key") {
		t.Fatalf("web-cookie error was returned instead of desktop error: %v", err)
	}
}

func TestExplicitEKeyTakesPriorityOverCookie(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		http.Error(writer, "must not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	replaceQQAPIForTest(t, server)

	key := []byte("12345678")
	plain := append([]byte("fLaC"), bytes.Repeat([]byte{0x2a}, 256)...)
	encrypted := append([]byte(nil), plain...)
	(&qmc2MapCipher{key: key}).decrypt(0, encrypted)
	data := makeMusicExData(encrypted, 1, "song-mid", "test.mflac")
	options := DecodeOptions{
		QQEKey:   base64.StdEncoding.EncodeToString(key),
		QQCookie: "qqmusic_key=secret",
	}
	got, err := decryptQMC2Data(context.Background(), data, "test.mflac", options, nil)
	if err != nil {
		t.Fatalf("decryptQMC2Data() error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("decrypted data does not match plaintext")
	}
	if calls.Load() != 0 {
		t.Fatalf("QQ API was called %d times", calls.Load())
	}
}

func replaceQQAPIForTest(t *testing.T, server *httptest.Server) {
	t.Helper()
	oldURL := qqMusicAPIURL
	oldClient := qqMusicHTTPClient
	qqMusicAPIURL = server.URL
	qqMusicHTTPClient = server.Client()
	t.Cleanup(func() {
		qqMusicAPIURL = oldURL
		qqMusicHTTPClient = oldClient
	})
}
