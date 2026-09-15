package appservice

// RealtimeFrameEventName 是原生端投递一条服务端实时事件的 Wails 事件名，事件数据为 RealtimeFrameEvent。
const RealtimeFrameEventName = "cervi:realtime:frame"

// RealtimeClosedEventName 是原生端实时事件流结束的 Wails 事件名，事件数据为 RealtimeClosedEvent。
const RealtimeClosedEventName = "cervi:realtime:closed"

// RealtimeConnection 是原生端本地实时连接编号，事件据此区分新旧连接。
type RealtimeConnection struct {
	ConnectionID string `json:"connectionId"`
}

// RealtimeFrameEvent 携带原生端收到的一条服务端实时事件 JSON 文本。
type RealtimeFrameEvent struct {
	ConnectionID string `json:"connectionId"`
	Frame        string `json:"frame"`
}

// RealtimeClosedEvent 表示原生端实时事件流已结束。
type RealtimeClosedEvent struct {
	ConnectionID string `json:"connectionId"`
}
