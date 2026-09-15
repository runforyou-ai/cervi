// Package protocol 定义实时 SSE 事件流的 JSON 事件契约与编解码，TypeScript 端对应 frontend/src/api/realtime/protocol.ts。
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
	// ErrUnknownFrame 表示未定义的事件种类，接收方忽略该事件。
	ErrUnknownFrame = errors.New("realtime protocol: unknown event type")
	// ErrUnsupportedVersion 表示事件的协议主版本与当前版本不一致。
	ErrUnsupportedVersion = errors.New("realtime protocol: unsupported version")
)

// Type 定义事件种类。
type Type string

const (
	TypeServerHello              Type = "server_hello"
	TypePing                     Type = "ping"
	TypeConversationChanged      Type = "conversation_changed"
	TypeConversationRemoved      Type = "conversation_removed"
	TypeConversationStateChanged Type = "conversation_state_changed"
	TypeIdentityProfileChanged   Type = "identity_profile_changed"
)

// Frame 是可编码的实时事件。
type Frame interface {
	// FrameType 返回事件种类。
	FrameType() Type
}

// ServerHello 返回连接编号和订阅安装后读取的同步探针值；变更通知可能先于本事件到达。
type ServerHello struct {
	ConnectionID string               `json:"connectionId"`
	SyncHeads    appservice.SyncHeads `json:"syncHeads"`
}

// Ping 是服务端定期发送的心跳，客户端据此判断事件流仍然存活。
type Ping struct{}

// ConversationChanged 表示会话变到了指定版本。
type ConversationChanged struct {
	ConversationID string `json:"conversationId"`
	Version        int64  `json:"version,string"`
}

// ConversationRemoved 表示当前用户失去指定会话的阅读资格。
type ConversationRemoved struct {
	ConversationID string `json:"conversationId"`
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

// FrameType 返回服务端 Hello 事件种类。
func (ServerHello) FrameType() Type { return TypeServerHello }

// FrameType 返回心跳事件种类。
func (Ping) FrameType() Type { return TypePing }

// FrameType 返回会话变更事件种类。
func (ConversationChanged) FrameType() Type { return TypeConversationChanged }

// FrameType 返回会话失权事件种类。
func (ConversationRemoved) FrameType() Type { return TypeConversationRemoved }

// FrameType 返回本人会话状态变更事件种类。
func (ConversationStateChanged) FrameType() Type { return TypeConversationStateChanged }

// FrameType 返回身份资料变更事件种类。
func (IdentityProfileChanged) FrameType() Type { return TypeIdentityProfileChanged }

// envelope 是事件在 SSE data 行中的外层结构。
type envelope struct {
	V    int             `json:"v"`
	Type Type            `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// decoder 把事件数据解码为具体事件。
type decoder func(json.RawMessage) (Frame, error)

// decoders 是已定义的事件种类。
var decoders = map[Type]decoder{
	TypeServerHello:              decodeAs[ServerHello],
	TypePing:                     decodeAs[Ping],
	TypeConversationChanged:      decodeAs[ConversationChanged],
	TypeConversationRemoved:      decodeAs[ConversationRemoved],
	TypeConversationStateChanged: decodeAs[ConversationStateChanged],
	TypeIdentityProfileChanged:   decodeAs[IdentityProfileChanged],
}

// Encode 把事件编码为带协议主版本的单行 JSON 文本。
func Encode(frame Frame) ([]byte, error) {
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("encode realtime event %s: %w", frame.FrameType(), err)
	}
	return json.Marshal(envelope{V: Version, Type: frame.FrameType(), Data: data})
}

// Decode 先校验协议主版本再按事件种类解码；只校验结构和类型，业务合法性由接收方判断。
func Decode(data []byte) (Frame, error) {
	var value envelope
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode realtime event: %w", err)
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
		return nil, fmt.Errorf("decode realtime event %s: %w", value.Type, err)
	}
	return frame, nil
}

// decodeAs 把事件数据解码为指定事件结构，缺少数据时返回零值事件。
func decodeAs[T Frame](data json.RawMessage) (Frame, error) {
	var frame T
	if len(data) == 0 {
		return frame, nil
	}
	err := json.Unmarshal(data, &frame)
	return frame, err
}
