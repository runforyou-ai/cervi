package appservice

// RealtimeFrameEventName 是原生端投递一帧服务端实时帧的 Wails 事件名，事件数据为 RealtimeFrameEvent。
const RealtimeFrameEventName = "cervi:realtime:frame"

// RealtimeClosedEventName 是原生端实时连接结束的 Wails 事件名，事件数据为 RealtimeClosedEvent。
const RealtimeClosedEventName = "cervi:realtime:closed"

// RealtimeConnectInput 定义原生端建立实时连接时声明的应用版本与能力集合。
type RealtimeConnectInput struct {
	AppVersion   string   `json:"appVersion"`
	Capabilities []string `json:"capabilities"`
}

// RealtimeConnection 是原生端本地实时连接编号，事件据此区分新旧连接。
type RealtimeConnection struct {
	ConnectionID string `json:"connectionId"`
}

// RealtimeFrameEvent 携带原生端收到的一帧服务端帧 JSON 文本。
type RealtimeFrameEvent struct {
	ConnectionID string `json:"connectionId"`
	Frame        string `json:"frame"`
}

// RealtimeClosedEvent 表示原生端实时连接结束，携带 WebSocket 关闭码与原因；未收到关闭帧时关闭码为 -1。
type RealtimeClosedEvent struct {
	ConnectionID string `json:"connectionId"`
	Code         int    `json:"code"`
	Reason       string `json:"reason"`
}
