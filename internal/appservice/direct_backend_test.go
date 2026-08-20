//go:build server

package appservice

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/common/fielderror"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestChannelContractConversion 验证渠道模型与应用服务契约之间的转换。
func TestChannelContractConversion(t *testing.T) {
	now := time.Now().UTC()
	description := "官网咨询"
	channel := websiteChannelFromModel(&servermodels.Channel{
		ID: "channel-1", OrganizationID: "organization-1", CreatedByUserID: "user-1",
		Type: string(ChannelTypeWebsite), Name: "产品官网", Description: &description,
		DefaultLocale: string(LocaleChineseSimplified), CreatedAt: now, UpdatedAt: now,
	})
	if channel.Type != ChannelTypeWebsite || channel.DefaultLocale != LocaleChineseSimplified || channel.Description == nil || *channel.Description != description {
		t.Fatalf("channel conversion = %#v", channel)
	}

	input := channelInput(WebsiteChannelInput{Name: "产品官网", Description: description, DefaultLocale: LocaleEnglishUnitedStates})
	if input.DefaultLocale != domain.LocaleEnglishUnitedStates || input.Name != "产品官网" {
		t.Fatalf("channel input conversion = %#v", input)
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
	want := S3Setting{
		Enabled: true, Provider: StorageProviderMinIO, Endpoint: "http://127.0.0.1:9000", Region: "us-east-1",
		Bucket: "cervi", AccessKeyID: "access", SecretAccessKey: "secret", ForcePathStyle: true,
	}
	got := s3SettingFromAction(s3SettingToAction(want))
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
		&fielderror.Error{Fields: map[string]fielderror.Code{"stage": contactaction.ValidationStageInvalid}},
		cervii18n.ErrorContactUpdateFailed,
	)
	applicationError, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if applicationError.Status != http.StatusBadRequest || applicationError.Code != "VALIDATION_FAILED" || applicationError.Fields["stage"] != "请选择有效的联系人阶段。" {
		t.Fatalf("application error = %#v", applicationError)
	}

	settingError := backend.s3SettingError(
		context.Background(),
		RequestMeta{Locale: LocaleEnglishUnitedStates},
		&fielderror.Error{Fields: map[string]fielderror.Code{"provider": settingaction.ValidationProviderInvalid}},
		cervii18n.ErrorS3SettingSaveFailed,
	)
	converted := settingError.(*Error)
	if converted.Status != http.StatusBadRequest || converted.Fields["provider"] == "" {
		t.Fatalf("setting error = %#v", converted)
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
