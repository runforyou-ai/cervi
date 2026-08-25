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
		AppProductName,
		AppTrayOpen,
		AppTrayQuit,
		AppMenuFile,
		AppMenuEdit,
		AppMenuView,
		AppMenuWindow,
		AppMenuHelp,
		AppMenuAbout,
		AppMenuServices,
		AppMenuHide,
		AppMenuHideOthers,
		AppMenuShowAll,
		AppMenuQuit,
		AppMenuClose,
		AppMenuUndo,
		AppMenuRedo,
		AppMenuCut,
		AppMenuCopy,
		AppMenuPaste,
		AppMenuPasteAndMatchStyle,
		AppMenuDelete,
		AppMenuSelectAll,
		AppMenuSpeech,
		AppMenuStartSpeaking,
		AppMenuStopSpeaking,
		AppMenuReload,
		AppMenuForceReload,
		AppMenuOpenDevTools,
		AppMenuActualSize,
		AppMenuZoomIn,
		AppMenuZoomOut,
		AppMenuToggleFullscreen,
		AppMenuMinimize,
		AppMenuZoom,
		AppMenuBringAllToFront,
		AppMenuLearnMore,
		DialogProfileImageTitle,
		DialogProfileImageChoose,
		MessengerPreviewTitle,
		MessengerPreviewStageLabel,
		MessengerDefaultTitle,
		MessengerUnavailableTitle,
		MessengerUnavailableMessage,
		MessengerHome,
		MessengerMessages,
		MessengerHelp,
		MessengerMessage,
		MessengerClose,
		MessengerAttach,
		MessengerEmoji,
		MessengerDefaultAgentName,
		MessengerDefaultAgentLastActive,
		MessengerDemoReply,
		MessengerWelcome,
		MessengerHowCanWeHelp,
		MessengerStartConversation,
		MessengerDefaultResponse,
		MessengerExploreHelp,
		MessengerExploreHelpDescription,
		MessengerViewAll,
		MessengerGettingStarted,
		MessengerGettingStartedDescription,
		MessengerFeaturesAndSettings,
		MessengerFeaturesDescription,
		MessengerCommonQuestions,
		MessengerQuestionsDescription,
		MessengerNoMessages,
		MessengerNoMessagesDescription,
		MessengerSearchHelp,
		MessengerCollections,
		MessengerCollectionCount,
		MessengerThreeArticles,
		MessengerFiveArticles,
		MessengerSixArticles,
		MessengerNoHelpResults,
		MessengerBack,
		MessengerArticleOneTitle,
		MessengerArticleOneBody,
		MessengerArticleTwoTitle,
		MessengerArticleTwoBody,
		MessengerArticleThreeTitle,
		MessengerArticleThreeBody,
		MessengerStillNeedHelp,
		MessengerConversationPrompt,
		MessengerMore,
		MessengerExpandWindow,
		MessengerCollapseWindow,
		MessengerRecordVoice,
		MessengerPlayVoice,
		MessengerPauseVoice,
		MessengerSend,
		MessengerCancelRecording,
		MessengerStopRecording,
		MessengerNavigation,
		MessengerLoading,
		MessengerRetry,
		MessengerRequestFailed,
		MessengerSessionWaiting,
		MessengerSessionActive,
		MessengerSessionPending,
		MessengerSessionClosed,
		ErrorInternal,
		ErrorMethodNotAllowed,
		ErrorInstallationStatusReadFailed,
		ErrorAlreadyInitialized,
		ErrorInstallationRequired,
		ErrorAuthenticationStatusFailed,
		ErrorAuthenticationRequired,
		ErrorValidationFailed,
		ErrorInstallationFailed,
		ErrorInvalidCredentials,
		ErrorLoginFailed,
		ErrorLogoutFailed,
		ErrorProfileUpdateFailed,
		ErrorPasswordUpdateFailed,
		ErrorServerURLInvalid,
		ErrorServerUnavailable,
		ErrorServerConnectionSaveFailed,
		ErrorServerConnectionRequired,
		ErrorServerInitializationRequired,
		ErrorRemoteRequestCreateFailed,
		ErrorServerConnectionFailed,
		ErrorWebsiteConversationNotFound,
		ErrorWebsiteMessengerLoadFailed,
		ErrorWebsiteMessageSendFailed,
		ErrorWebsiteMessageListFailed,
		ErrorWebsiteMessageConflict,
		FieldOrganizationNameRequired,
		FieldOrganizationNameTooLong,
		FieldDisplayNameRequired,
		FieldEmailInvalid,
		FieldEmailDuplicate,
		FieldPasswordTooShort,
		FieldPasswordTooLong,
		FieldCurrentPasswordIncorrect,
		FieldServerURLComplete,
		FieldServerURLBaseOnly,
		FieldServerURLHTTPSRequired,
		FieldServerURLNotCervi,
		FieldVisitorTokenInvalid,
		FieldChannelIDInvalid,
		FieldConversationIDInvalid,
		FieldClientMessageIDInvalid,
		FieldMessageBodyRequired,
		FieldMessageBodyTooLong,
		FieldMessageCursorInvalid,
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
