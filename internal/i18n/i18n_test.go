package i18n

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// TestLocaleFilesContainTheSameKeys 验证所有语言文件包含相同的文案键。
func TestLocaleFilesContainTheSameKeys(t *testing.T) {
	enUS := readLocaleKeys(t, "locales/en-US.json")
	zhCN := readLocaleKeys(t, "locales/zh-CN.json")
	if !reflect.DeepEqual(enUS, zhCN) {
		t.Fatalf("locale keys differ: en-US=%v zh-CN=%v", enUS, zhCN)
	}
}

// TestLocalizeMatchesRequestedLanguage 验证本地化器匹配请求语言并回退到英文。
func TestLocalizeMatchesRequestedLanguage(t *testing.T) {
	tests := []struct {
		acceptLanguage string
		wantMessage    string
		wantLanguage   string
	}{
		{acceptLanguage: "zh-CN", wantMessage: "请先登录。", wantLanguage: "zh-CN"},
		{acceptLanguage: "en-US", wantMessage: "Please log in first.", wantLanguage: "en-US"},
		{acceptLanguage: "fr-FR", wantMessage: "Please log in first.", wantLanguage: "en-US"},
	}

	for _, test := range tests {
		message, matchedLanguage := Localize(test.acceptLanguage, ErrorAuthenticationRequired)
		if message != test.wantMessage || matchedLanguage != test.wantLanguage {
			t.Fatalf(
				"Localize(%q) = (%q, %q), want (%q, %q)",
				test.acceptLanguage,
				message,
				matchedLanguage,
				test.wantMessage,
				test.wantLanguage,
			)
		}
	}
}

// TestAllMessageKeysCanBeLocalized 验证全部类型化文案键均存在中英文翻译。
func TestAllMessageKeysCanBeLocalized(t *testing.T) {
	keys := []Key{
		ErrorMethodNotAllowed,
		ErrorInstallationStatusReadFailed,
		ErrorAlreadyInitialized,
		ErrorInstallationRequired,
		ErrorAuthenticationStatusFailed,
		ErrorAuthenticationRequired,
		ErrorValidationFailed,
		ErrorInstallationFailed,
		ErrorAuthenticationInputInvalid,
		ErrorInvalidCredentials,
		ErrorLoginFailed,
		ErrorLogoutFailed,
		ErrorServerURLInvalid,
		ErrorServerConnectionCreateFailed,
		ErrorServerUnavailable,
		ErrorServerConnectionSaveFailed,
		ErrorServerConnectionRequired,
		ErrorRemoteRequestCreateFailed,
		ErrorServerConnectionFailed,
		FieldOrganizationNameRequired,
		FieldDisplayNameRequired,
		FieldEmailInvalid,
		FieldPasswordTooShort,
		FieldPasswordTooLong,
		FieldServerURLComplete,
		FieldServerURLBaseOnly,
		FieldServerURLHTTPSRequired,
		FieldServerURLNotCervi,
	}

	for _, language := range []string{"en-US", "zh-CN"} {
		for _, key := range keys {
			if message, _ := Localize(language, key); message == "" {
				t.Fatalf("Localize(%q, %q) returned an empty message", language, key)
			}
		}
	}
}

// readLocaleKeys 读取并排序指定语言文件中的文案键。
func readLocaleKeys(t *testing.T, path string) []string {
	t.Helper()
	content, err := localeFiles.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(map[string]json.RawMessage)
	if err := json.Unmarshal(content, &messages); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(messages))
	for key := range messages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
