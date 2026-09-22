package domain

// ServiceTimeouts 定义企业客服的未响应提醒、未响应回收与队列等待提醒时长，单位为分钟。
type ServiceTimeouts struct {
	ResponseReminderMinutes int
	ResponseReclaimMinutes  int
	QueueReminderMinutes    int
}

// DefaultServiceTimeouts 返回企业未设置时的超时时长。
func DefaultServiceTimeouts() ServiceTimeouts {
	return ServiceTimeouts{ResponseReminderMinutes: 5, ResponseReclaimMinutes: 15, QueueReminderMinutes: 5}
}
