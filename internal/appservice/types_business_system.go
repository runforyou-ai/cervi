package appservice

import "time"

// BusinessSystem 定义企业配置的业务系统。
type BusinessSystem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// BusinessSystemInput 定义业务系统可编辑字段。
type BusinessSystemInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Enabled     bool   `json:"enabled"`
}

// BusinessSystemList 定义企业业务系统列表。
type BusinessSystemList struct {
	BusinessSystems []BusinessSystem `json:"businessSystems"`
}
