//go:build server

package inbox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestInboxCursor 验证微秒与空时间无损往返，并拒绝失效排序版本和筛选。
func TestInboxCursor(t *testing.T) {
	identity := &servermodels.Identity{Organization: servermodels.Organization{ID: "organization"}, User: servermodels.User{ID: "user"}}
	input := LoadInput{
		Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewCoworkers,
		AssigneeIdentityID: "019d4e1c-40a5-77dd-82e6-6951f9957ba5", ChannelID: "019d4e1c-40a5-77dd-82e6-6951f9957ba7",
		ServiceStatus: domain.ServiceSessionStatusClosed,
	}
	activity := time.Date(2026, 9, 9, 0, 0, 0, 123456000, time.UTC)
	for _, value := range []*time.Time{nil, &activity} {
		point := inboxCursorPoint{ID: "019d4e1c-40a5-77dd-82e6-6951f9957ba6", LastActivityAt: value}
		encoded, err := encodeInboxCursor(identity, input, point)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeInboxCursor(encoded, identity, input)
		if err != nil || decoded.ID != point.ID || (decoded.LastActivityAt == nil) != (value == nil) || (value != nil && !decoded.LastActivityAt.Equal(*value)) {
			t.Fatalf("round trip=%+v err=%v", decoded, err)
		}
		// 接受的 UUID 大小写形式统一回数据库小写值，保证区间比较与锚点匹配一致。
		uppercase := *decoded
		uppercase.ID = strings.ToUpper(point.ID)
		data, err := json.Marshal(uppercase)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := decodeInboxCursor(base64.RawURLEncoding.EncodeToString(data), identity, input)
		if err != nil || canonical.ID != point.ID {
			t.Fatalf("uppercase cursor=%+v err=%v", canonical, err)
		}
		// 续页使用相同筛选条件和边界，页大小可独立调整。
		resized := input
		resized.Limit = 1
		if _, err := decodeInboxCursor(encoded, identity, resized); err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*inboxCursor){
			func(c *inboxCursor) { c.Version++ },
			func(c *inboxCursor) { c.OrganizationID = "another" },
			func(c *inboxCursor) { c.UserID = "another" },
			func(c *inboxCursor) { c.Scope = domain.InboxScopeAll },
			func(c *inboxCursor) { c.CustomerView = domain.CustomerInboxViewMine },
			func(c *inboxCursor) { c.AssigneeIdentityID = "" },
			func(c *inboxCursor) { c.ChannelID = "" },
			func(c *inboxCursor) { c.ServiceStatus = domain.ServiceSessionStatusOpen },
			func(c *inboxCursor) { c.Kinds = []domain.ConversationType{domain.ConversationTypeGroup} },
			func(c *inboxCursor) { c.ID = "bad" },
		} {
			changed := *decoded
			mutate(&changed)
			data, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeInboxCursor(base64.RawURLEncoding.EncodeToString(data), identity, input); !errors.Is(err, ErrCursorInvalid) {
				t.Fatalf("accepted invalid cursor=%+v err=%v", changed, err)
			}
		}
	}
	for _, value := range []string{"!", "bnVsbA", base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"lastActivityAt":"bad-time"}`))} {
		if _, err := decodeInboxCursor(value, identity, input); !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("accepted malformed cursor=%s err=%v", value, err)
		}
	}
}
