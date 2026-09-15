package protocol

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

// fixtureCase 是 Go 与 TypeScript 共用的帧夹具样例。
type fixtureCase struct {
	Name      string          `json:"name"`
	Direction string          `json:"direction"`
	Result    string          `json:"result"`
	Encode    bool            `json:"encode"`
	Wire      json.RawMessage `json:"wire"`
}

// expectedFrames 是结果为 frame 的夹具样例在 Go 端解码后应得到的帧。
var expectedFrames = map[string]Frame{
	"authenticate":                      Authenticate{Token: "cervi-token-example"},
	"client_hello":                      ClientHello{ClientKind: ClientWeb, AppVersion: "0.1.0"},
	"client_hello_capabilities":         ClientHello{ClientKind: ClientDesktop, AppVersion: "0.1.0", Capabilities: []string{"future_capability"}},
	"client_ping":                       Ping{},
	"client_pong":                       Pong{},
	"authenticated":                     Authenticated{},
	"server_hello":                      ServerHello{ConnectionID: "conn-01", SyncHeads: appservice.SyncHeads{ConversationCount: 3, ConversationChecksum: "18446744073709551615", IdentityProfileVersion: "9223372036854775807"}},
	"server_ping":                       Ping{},
	"server_pong":                       Pong{},
	"conversation_changed":              ConversationChanged{ConversationID: "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b", Version: 9223372036854775807},
	"conversation_state_changed":        ConversationStateChanged{ConversationID: "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b", Version: 42},
	"identity_profile_changed":          IdentityProfileChanged{Version: 9007199254740993},
	"conversation_removed":              ConversationRemoved{ConversationID: "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b"},
	"session_revoked":                   SessionRevoked{Reason: SessionRevokedLogout},
	"session_revoked_unknown_reason":    SessionRevoked{Reason: "future_reason"},
	"session_revoked_user_disabled":     SessionRevoked{Reason: SessionRevokedUserDisabled},
	"server_going_away":                 ServerGoingAway{},
	"realtime_error":                    RealtimeError{Code: ErrorAuthenticationFailed},
	"conversation_changed_extra_fields": ConversationChanged{ConversationID: "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b", Version: 7},
	"server_ping_without_data":          Ping{},
}

// TestFrameFixtures 按共用夹具校验 Go 端解码结果，并校验编码输出与夹具线上格式一致。
func TestFrameFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/frames.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var cases []fixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}

	encodedTypes := map[string]map[Type]bool{"client": {}, "server": {}}
	usedFrames := map[string]bool{}
	for _, fixture := range cases {
		t.Run(fixture.Name, func(t *testing.T) {
			decodeFrame := DecodeServer
			if fixture.Direction == "client" {
				decodeFrame = DecodeClient
			}
			frame, err := decodeFrame(fixture.Wire)
			switch fixture.Result {
			case "frame":
				expected, ok := expectedFrames[fixture.Name]
				if !ok {
					t.Fatalf("missing expected frame")
				}
				usedFrames[fixture.Name] = true
				if err != nil {
					t.Fatalf("decode: %v", err)
				}
				if !reflect.DeepEqual(frame, expected) {
					t.Fatalf("decoded %#v, want %#v", frame, expected)
				}
				if !fixture.Encode {
					return
				}
				encodedTypes[fixture.Direction][frame.FrameType()] = true
				encoded, err := Encode(expected)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				var got, want any
				if err := json.Unmarshal(encoded, &got); err != nil {
					t.Fatalf("parse encoded frame: %v", err)
				}
				if err := json.Unmarshal(fixture.Wire, &want); err != nil {
					t.Fatalf("parse fixture wire: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("encoded %s, want %s", encoded, fixture.Wire)
				}
			case "ignored":
				if !errors.Is(err, ErrUnknownFrame) {
					t.Fatalf("decode error %v, want ErrUnknownFrame", err)
				}
			case "unsupported_version":
				if !errors.Is(err, ErrUnsupportedVersion) {
					t.Fatalf("decode error %v, want ErrUnsupportedVersion", err)
				}
			case "invalid":
				if err == nil || errors.Is(err, ErrUnknownFrame) || errors.Is(err, ErrUnsupportedVersion) {
					t.Fatalf("decode error %v, want invalid frame error", err)
				}
			default:
				t.Fatalf("unknown fixture result %q", fixture.Result)
			}
		})
	}

	// 每个方向的每种帧都至少有一个编码往返样例，每个期望帧都对应夹具样例。
	for direction, decoders := range map[string]map[Type]decoder{"client": clientDecoders, "server": serverDecoders} {
		for frameType := range decoders {
			if !encodedTypes[direction][frameType] {
				t.Errorf("%s frame %s has no encode fixture", direction, frameType)
			}
		}
	}
	for name := range expectedFrames {
		if !usedFrames[name] {
			t.Errorf("expected frame %s has no fixture", name)
		}
	}
}
