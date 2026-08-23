//go:build server

package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/common/embedhost"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestNormalizeWebsiteChannelInput 验证网站渠道字段规范化和长度限制。
func TestNormalizeWebsiteChannelInput(t *testing.T) {
	normalized, fields := normalizeWebsiteChannelInput(WebsiteChannelInput{
		Type:          domain.ChannelTypeWebsite,
		Name:          "  产品官网  ",
		Description:   "  接收访客咨询  ",
		DefaultLocale: " zh-CN ",
	})
	if len(fields) != 0 {
		t.Fatalf("validation fields = %#v, want empty", fields)
	}
	if normalized.Name != "产品官网" || normalized.Description != "接收访客咨询" || normalized.DefaultLocale != domain.LocaleChineseSimplified {
		t.Fatalf("unexpected normalized input: %#v", normalized)
	}

	_, fields = normalizeWebsiteChannelInput(WebsiteChannelInput{
		Type:          "email",
		Name:          strings.Repeat("鹿", 101),
		Description:   strings.Repeat("行", 2001),
		DefaultLocale: "fr-FR",
	})
	if fields["type"] != ValidationTypeInvalid {
		t.Fatalf("type validation = %q, want %q", fields["type"], ValidationTypeInvalid)
	}
	if fields["name"] != ValidationNameTooLong {
		t.Fatalf("name validation = %q, want %q", fields["name"], ValidationNameTooLong)
	}
	if fields["description"] != ValidationDescriptionTooLong {
		t.Fatalf("description validation = %q, want %q", fields["description"], ValidationDescriptionTooLong)
	}
	if fields["defaultLocale"] != ValidationDefaultLocaleInvalid {
		t.Fatalf("default locale validation = %q, want %q", fields["defaultLocale"], ValidationDefaultLocaleInvalid)
	}
}

// TestNormalizeWebsiteChannelInputCountsUnicodeCodePoints 验证补充平面字符按码点计数。
func TestNormalizeWebsiteChannelInputCountsUnicodeCodePoints(t *testing.T) {
	_, fields := normalizeWebsiteChannelInput(WebsiteChannelInput{
		Type:          domain.ChannelTypeWebsite,
		Name:          strings.Repeat("😀", 100),
		Description:   strings.Repeat("😀", 2000),
		DefaultLocale: domain.LocaleChineseSimplified,
	})
	if len(fields) != 0 {
		t.Fatalf("validation fields = %#v, want empty", fields)
	}

	_, fields = normalizeWebsiteChannelInput(WebsiteChannelInput{
		Type:          domain.ChannelTypeWebsite,
		Name:          strings.Repeat("😀", 101),
		Description:   strings.Repeat("😀", 2001),
		DefaultLocale: domain.LocaleChineseSimplified,
	})
	if fields["name"] != ValidationNameTooLong || fields["description"] != ValidationDescriptionTooLong {
		t.Fatalf("unexpected validation fields: %#v", fields)
	}
}

// TestNormalizeWebsiteChannelChatInterfaceInput 验证聊天界面字段规范化和校验。
func TestNormalizeWebsiteChannelChatInterfaceInput(t *testing.T) {
	normalized, fields := normalizeWebsiteChannelChatInterfaceInput(WebsiteChannelChatInterfaceInput{
		Title:           " 在线咨询 ",
		Subtitle:        " 通常会很快回复 ",
		GreetingMessage: " 你好 ",
		ThemeColor:      " #16a34a ",
	})
	if len(fields) != 0 {
		t.Fatalf("validation fields = %#v, want empty", fields)
	}
	if normalized.Title != "在线咨询" || normalized.Subtitle != "通常会很快回复" || normalized.GreetingMessage != "你好" || normalized.ThemeColor != "#16A34A" {
		t.Fatalf("unexpected normalized chat interface: %#v", normalized)
	}

	_, fields = normalizeWebsiteChannelChatInterfaceInput(WebsiteChannelChatInterfaceInput{
		Title:           strings.Repeat("鹿", 101),
		Subtitle:        strings.Repeat("行", 121),
		GreetingMessage: strings.Repeat("聊", 501),
		ThemeColor:      "blue",
	})
	if fields["title"] != ValidationChatTitleTooLong || fields["subtitle"] != ValidationChatSubtitleTooLong || fields["greetingMessage"] != ValidationGreetingTooLong || fields["themeColor"] != ValidationThemeColorInvalid {
		t.Fatalf("unexpected validation fields: %#v", fields)
	}
}

// TestNormalizeWebsiteChannelAccessInput 验证允许网站配置的规范化和数量限制。
func TestNormalizeWebsiteChannelAccessInput(t *testing.T) {
	normalized, fields := normalizeWebsiteChannelAccessInput(WebsiteChannelAccessInput{
		AllowedHosts: []string{" Example.COM ", "*.example.com"},
	})
	if len(fields) != 0 || len(normalized.AllowedHosts) != 2 || normalized.AllowedHosts[0] != "example.com" {
		t.Fatalf("normalized access = %#v, fields = %#v", normalized, fields)
	}

	_, fields = normalizeWebsiteChannelAccessInput(WebsiteChannelAccessInput{AllowedHosts: []string{"not a host"}})
	if fields["allowedHosts"] != ValidationAllowedHostInvalid {
		t.Fatalf("invalid host validation = %#v", fields)
	}

	tooMany := make([]string, embedhost.MaxHosts+1)
	_, fields = normalizeWebsiteChannelAccessInput(WebsiteChannelAccessInput{AllowedHosts: tooMany})
	if fields["allowedHosts"] != ValidationAllowedHostsTooMany {
		t.Fatalf("host count validation = %#v", fields)
	}
}

// TestMalformedChannelIDReturnsNotFound 验证非法 UUID 不会进入数据库查询。
func TestMalformedChannelIDReturnsNotFound(t *testing.T) {
	identity := &servermodels.Identity{}
	input := WebsiteChannelInput{
		Type:          domain.ChannelTypeWebsite,
		Name:          "产品官网",
		DefaultLocale: domain.LocaleChineseSimplified,
	}

	tests := []struct {
		name    string
		execute func() error
	}{
		{
			name: "get",
			execute: func() error {
				_, err := NewGetWebsiteChannelQuery(nil).Execute(context.Background(), identity, "not-a-uuid")
				return err
			},
		},
		{
			name: "update",
			execute: func() error {
				_, err := NewUpdateWebsiteChannelAction(nil).Execute(context.Background(), identity, "not-a-uuid", input)
				return err
			},
		},
		{
			name: "update chat interface",
			execute: func() error {
				_, err := NewUpdateWebsiteChannelChatInterfaceAction(nil).Execute(context.Background(), identity, "not-a-uuid", WebsiteChannelChatInterfaceInput{})
				return err
			},
		},
		{
			name: "update access",
			execute: func() error {
				_, err := NewUpdateWebsiteChannelAccessAction(nil).Execute(context.Background(), identity, "not-a-uuid", WebsiteChannelAccessInput{})
				return err
			},
		},
		{
			name: "update status",
			execute: func() error {
				_, err := NewUpdateWebsiteChannelStatusAction(nil).Execute(context.Background(), identity, "not-a-uuid", false)
				return err
			},
		},
		{
			name: "get public",
			execute: func() error {
				_, err := NewGetPublicWebsiteChannelQuery(nil).Execute(context.Background(), "not-a-uuid")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.execute(); !errors.Is(err, ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}
