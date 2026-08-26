package appservice

// MemberOption 定义可分配的企业身份选择项。
type MemberOption struct {
	ID          string                   `json:"id"`
	Type        OrganizationIdentityType `json:"type"`
	DisplayName string                   `json:"displayName"`
	AvatarURL   string                   `json:"avatarUrl"`
}

// MemberOptionListInput 定义企业身份选择项查询条件。
type MemberOptionListInput struct {
	Query    string `json:"query" query:"query"`
	Page     int    `json:"page" query:"page,default=1"`
	PageSize int    `json:"pageSize" query:"pageSize,default=50"`
}

// MemberOptionList 定义企业身份选择项分页结果。
type MemberOptionList struct {
	Members []MemberOption `json:"members"`
	Page    PageInfo       `json:"page"`
}
