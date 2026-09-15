// Package protocol 定义实时 WebSocket 连接的 JSON 帧契约与编解码，TypeScript 端对应 frontend/src/api/realtime/protocol.ts。
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

// Version 是当前协议主版本，只有破坏性演进才提升。
const Version = 1

var (
	// ErrUnknownFrame 表示当前方向未定义的帧种类，接收方忽略该帧。
	ErrUnknownFrame = errors.New("realtime protocol: unknown frame type")
	// ErrUnsupportedVersion 表示帧的协议主版本与当前版本不一致。
	ErrUnsupportedVersion = errors.New("realtime protocol: unsupported version")
)

// Type 定义帧种类。
type Type string

const (
	TypeAuthenticate             Type = "authenticate"
	TypeClientHello              Type = "client_hello"
	TypePing                     Type = "ping"
	TypePong                     Type = "pong"
	TypeAuthenticated            Type = "authenticated"
	TypeServerHello              Type = "server_hello"
	TypeConversationChanged      Type = "conversation_changed"
	TypeConversationStateChanged Type = "conversation_state_changed"
	TypeIdentityProfileChanged   Type = "identity_profile_changed"
	TypeAccessRevoked            Type = "access_revoked"
	TypeSessionRevoked           Type = "session_revoked"
	TypeServerGoingAway          Type = "server_going_away"
	TypeRealtimeError            Type = "realtime_error"
)

// ClientKind 定义发起连接的客户端种类。
type ClientKind string

const (
	ClientWeb       ClientKind = "web"
	ClientDesktop   ClientKind = "desktop"
	ClientMobile    ClientKind = "mobile"
	ClientMessenger ClientKind = "messenger"
)

// SessionRevokedReason 定义登录会话被撤销的原因，接收方按未知原因处理未定义的值。
type SessionRevokedReason string

const (
	SessionRevokedLogout       SessionRevokedReason = "logout"
	SessionRevokedUserDisabled SessionRevokedReason = "user_disabled"
)

// ErrorCode 定义连接级错误码，接收方按未知错误处理未定义的值。
type ErrorCode string

const (
	ErrorUnsupportedVersion    ErrorCode = "unsupported_version"
	ErrorInvalidFrame          ErrorCode = "invalid_frame"
	ErrorAuthenticationFailed  ErrorCode = "authentication_failed"
	ErrorAuthenticationTimeout ErrorCode = "authentication_timeout"
)

// Frame 是可编码的实时帧。
type Frame interface {
	// FrameType 返回帧种类。
	FrameType() Type
}

// Authenticate 是连接建立后的首帧，携带与业务调用相同的登录令牌。
type Authenticate struct {
	Token string `json:"token"`
}

// ClientHello 在认证成功后声明客户端种类、应用版本和能力集合。
type ClientHello struct {
	ClientKind   ClientKind `json:"clientKind"`
	AppVersion   string     `json:"appVersion"`
	Capabilities []string   `json:"capabilities,omitempty"`
}

// Ping 请求对端回复 Pong。
type Ping struct{}

// Pong 回复 Ping。
type Pong struct{}

// Authenticated 表示首帧认证成功。
type Authenticated struct{}

// ServerHello 返回连接信息、服务端能力和订阅安装后读取的同步探针值。
type ServerHello struct {
	ConnectionID string               `json:"connectionId"`
	Capabilities []string             `json:"capabilities,omitempty"`
	SyncHeads    appservice.SyncHeads `json:"syncHeads"`
}

// ConversationChanged 表示会话变到了指定版本。
type ConversationChanged struct {
	ConversationID string `json:"conversationId"`
	Version        int64  `json:"version,string"`
}

// ConversationStateChanged 表示本人对会话的个人状态变到了指定版本。
type ConversationStateChanged struct {
	ConversationID string `json:"conversationId"`
	Version        int64  `json:"version,string"`
}

// IdentityProfileChanged 表示本人身份资料变到了指定版本。
type IdentityProfileChanged struct {
	Version int64 `json:"version,string"`
}

// AccessRevoked 表示连接失去指定会话的阅读资格。
type AccessRevoked struct {
	ConversationID string `json:"conversationId"`
}

// SessionRevoked 表示连接所属登录会话已失效，服务端随后关闭连接。
type SessionRevoked struct {
	Reason SessionRevokedReason `json:"reason"`
}

// ServerGoingAway 表示服务端即将下线，客户端应重新连接。
type ServerGoingAway struct{}

// RealtimeError 表示连接级错误。
type RealtimeError struct {
	Code ErrorCode `json:"code"`
}

// FrameType 返回认证帧种类。
func (Authenticate) FrameType() Type { return TypeAuthenticate }

// FrameType 返回客户端 Hello 帧种类。
func (ClientHello) FrameType() Type { return TypeClientHello }

// FrameType 返回 Ping 帧种类。
func (Ping) FrameType() Type { return TypePing }

// FrameType 返回 Pong 帧种类。
func (Pong) FrameType() Type { return TypePong }

// FrameType 返回认证成功帧种类。
func (Authenticated) FrameType() Type { return TypeAuthenticated }

// FrameType 返回服务端 Hello 帧种类。
func (ServerHello) FrameType() Type { return TypeServerHello }

// FrameType 返回会话变更帧种类。
func (ConversationChanged) FrameType() Type { return TypeConversationChanged }

// FrameType 返回本人会话状态变更帧种类。
func (ConversationStateChanged) FrameType() Type { return TypeConversationStateChanged }

// FrameType 返回身份资料变更帧种类。
func (IdentityProfileChanged) FrameType() Type { return TypeIdentityProfileChanged }

// FrameType 返回会话访问撤销帧种类。
func (AccessRevoked) FrameType() Type { return TypeAccessRevoked }

// FrameType 返回登录会话撤销帧种类。
func (SessionRevoked) FrameType() Type { return TypeSessionRevoked }

// FrameType 返回服务端下线帧种类。
func (ServerGoingAway) FrameType() Type { return TypeServerGoingAway }

// FrameType 返回连接级错误帧种类。
func (RealtimeError) FrameType() Type { return TypeRealtimeError }

// envelope 是帧在连接上的外层结构。
type envelope struct {
	V    int             `json:"v"`
	Type Type            `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// decoder 把帧数据解码为具体帧。
type decoder func(json.RawMessage) (Frame, error)

// clientDecoders 是客户端发往服务端的帧种类。
var clientDecoders = map[Type]decoder{
	TypeAuthenticate: decodeAs[Authenticate],
	TypeClientHello:  decodeAs[ClientHello],
	TypePing:         decodeAs[Ping],
	TypePong:         decodeAs[Pong],
}

// serverDecoders 是服务端发往客户端的帧种类。
var serverDecoders = map[Type]decoder{
	TypeAuthenticated:            decodeAs[Authenticated],
	TypeServerHello:              decodeAs[ServerHello],
	TypePing:                     decodeAs[Ping],
	TypePong:                     decodeAs[Pong],
	TypeConversationChanged:      decodeAs[ConversationChanged],
	TypeConversationStateChanged: decodeAs[ConversationStateChanged],
	TypeIdentityProfileChanged:   decodeAs[IdentityProfileChanged],
	TypeAccessRevoked:            decodeAs[AccessRevoked],
	TypeSessionRevoked:           decodeAs[SessionRevoked],
	TypeServerGoingAway:          decodeAs[ServerGoingAway],
	TypeRealtimeError:            decodeAs[RealtimeError],
}

// Encode 把帧编码为带协议主版本的 JSON 文本。
func Encode(frame Frame) ([]byte, error) {
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("encode realtime frame %s: %w", frame.FrameType(), err)
	}
	return json.Marshal(envelope{V: Version, Type: frame.FrameType(), Data: data})
}

// DecodeClient 解码客户端发往服务端的帧。
func DecodeClient(data []byte) (Frame, error) {
	return decode(data, clientDecoders)
}

// DecodeServer 解码服务端发往客户端的帧。
func DecodeServer(data []byte) (Frame, error) {
	return decode(data, serverDecoders)
}

// decode 先校验协议主版本再按帧种类解码；只校验结构和类型，业务合法性由接收方判断。
func decode(data []byte, decoders map[Type]decoder) (Frame, error) {
	var value envelope
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode realtime frame: %w", err)
	}
	if value.V != Version {
		return nil, ErrUnsupportedVersion
	}
	decodeData, ok := decoders[value.Type]
	if !ok {
		return nil, ErrUnknownFrame
	}
	frame, err := decodeData(value.Data)
	if err != nil {
		return nil, fmt.Errorf("decode realtime frame %s: %w", value.Type, err)
	}
	return frame, nil
}

// decodeAs 把帧数据解码为指定帧结构，缺少数据时返回零值帧。
func decodeAs[T Frame](data json.RawMessage) (Frame, error) {
	var frame T
	if len(data) == 0 {
		return frame, nil
	}
	err := json.Unmarshal(data, &frame)
	return frame, err
}
