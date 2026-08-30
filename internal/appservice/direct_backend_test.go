//go:build server

package appservice

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestChannelContractConversion 验证渠道模型与应用服务契约之间的转换。
func TestChannelContractConversion(t *testing.T) {
	now := time.Now().UTC()
	description := "官网咨询"
	channel := messageChannelFromRecord(&channelaction.MessageChannelRecord{
		ID: "channel-1", OrganizationID: "organization-1", CreatedByUserID: "user-1",
		Type: string(ChannelTypeWebsite), Name: "产品官网", Description: &description,
		DefaultLocale: string(LocaleChineseSimplified), Enabled: true, CreatedAt: now, UpdatedAt: now,
	})
	if channel.Type != ChannelTypeWebsite || channel.DefaultLocale != LocaleChineseSimplified || !channel.Enabled || channel.Description == nil || *channel.Description != description {
		t.Fatalf("channel conversion = %#v", channel)
	}

	input := channelInput(MessageChannelInput{
		Name: "产品官网", Description: description, DefaultLocale: LocaleEnglishUnitedStates,
		NewConversationTarget: ChannelRoutingTarget{Type: ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        ChannelRoutingTarget{Type: ChannelRoutingTargetTypePublicQueue},
	})
	if input.DefaultLocale != domain.LocaleEnglishUnitedStates || input.Name != "产品官网" || input.NewConversationTarget.Type != domain.ChannelRoutingTargetTypePublicQueue || input.FallbackTarget.Type != domain.ChannelRoutingTargetTypePublicQueue {
		t.Fatalf("channel input conversion = %#v", input)
	}
	createInput := createChannelInput(CreateMessageChannelInput{MessageChannelInput: MessageChannelInput{Name: "Telegram 客服"}, Type: ChannelTypeTelegram})
	if createInput.Type != domain.ChannelTypeTelegram || createInput.Name != "Telegram 客服" {
		t.Fatalf("channel create input conversion = %#v", createInput)
	}
	wechatCreateInput := createChannelInput(CreateMessageChannelInput{MessageChannelInput: MessageChannelInput{Name: "微信公众号客服"}, Type: ChannelTypeWeChatOfficialAccount})
	if wechatCreateInput.Type != domain.ChannelTypeWeChatOfficialAccount || wechatCreateInput.Name != "微信公众号客服" {
		t.Fatalf("wechat official account create input conversion = %#v", wechatCreateInput)
	}
}

// TestContactContractConversion 验证联系人输入和详情的枚举及切片转换。
func TestContactContractConversion(t *testing.T) {
	input := contactInput(ContactInput{
		DisplayName: "访客", ChannelID: "channel-1", Stage: ContactStageLead, Notes: "重点跟进",
		Methods: []ContactMethodInput{{Type: ContactMethodTypeEmail, Value: "visitor@example.com", Label: "工作", IsPrimary: true}},
	})
	if input.Stage != domain.ContactStageLead || len(input.Methods) != 1 || input.Methods[0].Type != domain.ContactMethodTypeEmail {
		t.Fatalf("contact input conversion = %#v", input)
	}

	displayName := "访客"
	label := "工作"
	detail := contactFromAction(&contactaction.ContactDetail{
		Contact: contactaction.ContactRecord{
			ID: "contact-1", SourceChannelID: "channel-1", DisplayName: &displayName,
			Stage: domain.ContactStageCustomer, CreatedAt: time.Now().UTC(),
		},
		SourceChannel:     contactaction.SourceChannel{ID: "channel-1", Type: domain.ChannelTypeWebsite, Name: "产品官网"},
		Methods:           []contactaction.ContactMethod{{Type: domain.ContactMethodTypeEmail, Value: "visitor@example.com", Label: &label, IsPrimary: true}},
		ChannelIdentities: []contactaction.ChannelIdentity{{ChannelID: "channel-1", ChannelName: "产品官网", ExternalID: "visitor-1"}},
	})
	if detail.Contact.Stage != ContactStageCustomer || detail.SourceChannel.Type != ChannelTypeWebsite || len(detail.Methods) != 1 || detail.Methods[0].Type != ContactMethodTypeEmail || len(detail.ChannelIdentities) != 1 {
		t.Fatalf("contact detail conversion = %#v", detail)
	}
}

// TestS3SettingContractRoundTrip 验证对象存储配置转换不丢失字段。
func TestS3SettingContractRoundTrip(t *testing.T) {
	input := S3SettingInput{
		Enabled: true, Provider: StorageProviderMinIO, Endpoint: "http://127.0.0.1:9000", Region: "us-east-1",
		Bucket: "cervi", AccessKeyID: "access", SecretAccessKey: "secret", ForcePathStyle: true,
	}
	want := S3Setting{
		Enabled: true, Provider: StorageProviderMinIO, Endpoint: "http://127.0.0.1:9000", Region: "us-east-1",
		Bucket: "cervi", AccessKeyID: "access", SecretAccessKey: "secret", ForcePathStyle: true,
	}
	got := s3SettingFromAction(s3SettingToAction(input))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("S3 setting round trip = %#v, want %#v", got, want)
	}
}

// TestDirectBackendMapsValidationErrors 验证直接后端保留结构化、本地化字段错误。
func TestDirectBackendMapsValidationErrors(t *testing.T) {
	backend := &DirectBackend{}
	err := backend.contactMutationError(
		context.Background(),
		RequestMeta{Locale: LocaleChineseSimplified},
		&common.FieldError{Fields: map[string]common.FieldCode{"stage": contactaction.ValidationStageInvalid}},
		cervii18n.ErrorContactUpdateFailed,
	)
	applicationError, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if applicationError.Kind != ErrorKindInvalid || applicationError.HTTPStatus() != http.StatusBadRequest || applicationError.Fields["stage"] != "请选择有效的联系人阶段。" {
		t.Fatalf("application error = %#v", applicationError)
	}

	settingError := backend.s3SettingError(
		context.Background(),
		RequestMeta{Locale: LocaleEnglishUnitedStates},
		&common.FieldError{Fields: map[string]common.FieldCode{"provider": settingaction.ValidationProviderInvalid}},
		cervii18n.ErrorS3SettingSaveFailed,
	)
	converted := settingError.(*Error)
	if converted.Kind != ErrorKindInvalid || converted.HTTPStatus() != http.StatusBadRequest || converted.Fields["provider"] == "" {
		t.Fatalf("setting error = %#v", converted)
	}

	passwordFields := passwordFieldKeys(map[string]common.FieldCode{
		"currentPassword": useraction.ValidationCurrentPasswordIncorrect,
		"newPassword":     useraction.ValidationPasswordTooShort,
	})
	if passwordFields["currentPassword"] != cervii18n.FieldCurrentPasswordIncorrect || passwordFields["newPassword"] != cervii18n.FieldPasswordTooShort {
		t.Fatalf("password field keys = %#v", passwordFields)
	}

	organizationFields := organizationFieldKeys(map[string]common.FieldCode{
		"name": organizationaction.ValidationNameRequired,
	})
	if organizationFields["name"] != cervii18n.FieldOrganizationNameRequired {
		t.Fatalf("organization field keys = %#v", organizationFields)
	}
}

// TestDirectBackendPreservesCancellation 验证请求取消不会转换为业务错误。
func TestDirectBackendPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (&DirectBackend{}).contactError(ctx, RequestMeta{}, errors.New("query failed"), cervii18n.ErrorContactReadFailed)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

// TestKnowledgeDocumentReadMapsRemoteNotFound 验证 Dify 文档 404 转换为文档不存在。
func TestKnowledgeDocumentReadMapsRemoteNotFound(t *testing.T) {
	backend := &DirectBackend{}
	err := backend.knowledgeDocumentReadError(
		context.Background(),
		RequestMeta{Locale: LocaleChineseSimplified},
		connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureNotFound, errors.New("not found")),
		cervii18n.ErrorKnowledgeDocumentReadFailed,
		"organization-1",
		"knowledge-base-1",
		"document-1",
	)
	converted, ok := err.(*Error)
	if !ok || converted.Kind != ErrorKindNotFound || converted.HTTPStatus() != http.StatusNotFound ||
		converted.Message != "没有找到这个知识文档。" {
		t.Fatalf("error = %#v", err)
	}
}
