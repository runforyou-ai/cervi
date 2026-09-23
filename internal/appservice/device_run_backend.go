package appservice

import "context"

// DeviceRunBackend 定义本机设备执行 Agent 运行的运行期调用。
//
// 每个方法必须携带一条 cervi:route 指令，格式为：
//
//	cervi:route <HTTP方法> <路径> [status=201] [query=<参数名>]
//
// appservicegen 按指令生成设备认证分发、Gin 路由与 Handler 和原生端 API Proxy 转发，
// 生成结果写入 device_run_direct_backend_gen.go、internal/api/device_run_service_gen.go
// 和 internal/apiproxy/device_run_backend_gen.go，不生成 Service 委托和前端绑定。
// 指令只接受 status 和 query 选项，每个调用先校验登录令牌，再校验 DeviceHeader
// 指向的设备属于当前用户且未撤销。
//
// 本契约的消费者是原生端的设备执行循环，界面发起的设备、工作区与助理工作区指定属于
// Backend，新增方法按消费者归入其中一个。
type DeviceRunBackend interface {
	// GetDeviceWork 返回本设备的工作水位与待领取运行。
	//cervi:route GET /devices/current/work
	GetDeviceWork(context.Context, RequestMeta) (DeviceWork, error)
	// ClaimDeviceRun 领取派发给本设备的排队运行并取得租约。
	//cervi:route POST /agent-runs/:runID/claim
	ClaimDeviceRun(context.Context, RequestMeta, string) (DeviceRunClaim, error)
	// RenewDeviceRunLease 为本设备持有的运行续租，运行已结束时返回 ended。
	//cervi:route POST /agent-runs/:runID/lease
	RenewDeviceRunLease(context.Context, RequestMeta, string) (DeviceRunLease, error)
	// PeekDeviceRunInputs 返回本设备持有运行尚未认领的输入信号。
	//cervi:route GET /agent-runs/:runID/inputs
	PeekDeviceRunInputs(context.Context, RequestMeta, string, DeviceRunInputPeekInput) (DeviceRunInputSignals, error)
	// ClaimDeviceRunInputs 为本设备持有的运行认领输入并返回截至该边界的上下文消息。
	//cervi:route POST /agent-runs/:runID/inputs/claim
	ClaimDeviceRunInputs(context.Context, RequestMeta, string, DeviceRunInputClaimInput) (DeviceRunClaimedInput, error)
	// CompleteDeviceRun 以成功结果收尾本设备持有的运行。
	//cervi:route POST /agent-runs/:runID/result
	CompleteDeviceRun(context.Context, RequestMeta, string, DeviceRunResultInput) error
	// FailDeviceRun 以失败原因收尾派发给本设备的运行。
	//cervi:route POST /agent-runs/:runID/failure
	FailDeviceRun(context.Context, RequestMeta, string, DeviceRunFailureInput) error
}
