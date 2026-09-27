package domain

// ContactFieldType 定义联系人字段类型。
type ContactFieldType string

const (
	ContactFieldTypeText   ContactFieldType = "text"
	ContactFieldTypeNumber ContactFieldType = "number"
	ContactFieldTypeDate   ContactFieldType = "date"
	ContactFieldTypeSelect ContactFieldType = "select"
)

// ContactFieldOption 定义单选字段的一个选项；取值保存选项编号。
type ContactFieldOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ContactProfileSource 定义联系人字段取值和标签的来源。
type ContactProfileSource string

const (
	ContactProfileSourceMember ContactProfileSource = "member"
)

const (
	// ContactFieldNameMaxLength 是联系人字段名称的最大字符数。
	ContactFieldNameMaxLength = 50
	// ContactFieldOptionNameMaxLength 是单选选项名称的最大字符数。
	ContactFieldOptionNameMaxLength = 50
	// ContactFieldValueMaxLength 是联系人字段取值的最大字符数。
	ContactFieldValueMaxLength = 500
	// ContactTagNameMaxLength 是联系人标签名称的最大字符数。
	ContactTagNameMaxLength = 30
)
