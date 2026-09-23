package appservice

import "time"

// BusinessHoursPeriod 定义一天内的一个工作时段，起止为 HH:mm，结束可取 24:00。
type BusinessHoursPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// BusinessHoursOverride 定义按日期覆盖的工作时段，Periods 为空表示当天休息。
type BusinessHoursOverride struct {
	Date    string                `json:"date"`
	Periods []BusinessHoursPeriod `json:"periods"`
}

// BusinessHours 定义企业客服工作时间；Weekly 固定 7 项，从周一到周日排列。
type BusinessHours struct {
	Enabled   bool                    `json:"enabled"`
	TimeZone  string                  `json:"timeZone"`
	Weekly    [][]BusinessHoursPeriod `json:"weekly"`
	Overrides []BusinessHoursOverride `json:"overrides"`
}

// ServiceTimeouts 定义企业客服的超时时长，单位为分钟：负责人未回复的提醒与回收时长、队列等待提醒时长，以及 AI 负责时客户未回复的跟进与关单时长。
type ServiceTimeouts struct {
	ResponseReminderMinutes int `json:"responseReminderMinutes"`
	ResponseReclaimMinutes  int `json:"responseReclaimMinutes"`
	QueueReminderMinutes    int `json:"queueReminderMinutes"`
	AIFollowUpMinutes       int `json:"aiFollowUpMinutes"`
	AICloseMinutes          int `json:"aiCloseMinutes"`
}

// ServiceCategoryInput 定义咨询分类可编辑字段；TeamID 为空表示转人工时按渠道失败路由。
type ServiceCategoryInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	TeamID      *string `json:"teamId"`
}

// ServiceCategory 定义咨询分类及其承接团队。
type ServiceCategory struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Team        *TeamSummary `json:"team"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// ServiceCategoryList 定义企业咨询分类目录。
type ServiceCategoryList struct {
	Categories []ServiceCategory `json:"categories"`
}
