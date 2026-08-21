//go:build server

package team

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// Input 定义团队可编辑字段。
type Input struct {
	Name        string
	Description string
}

// ListInput 定义团队列表查询条件。
type ListInput struct {
	Query    string
	Page     int
	PageSize int
}

// TeamRecord 定义团队详情字段。
type TeamRecord struct {
	ID          string    `bun:"id"`
	Name        string    `bun:"name"`
	Description string    `bun:"description"`
	MemberCount int       `bun:"member_count"`
	CreatedAt   time.Time `bun:"created_at"`
	UpdatedAt   time.Time `bun:"updated_at"`
}

// PageInfo 定义分页信息。
type PageInfo struct {
	Number int
	Size   int
	Total  int
}

// ListOutput 定义团队分页结果。
type ListOutput struct {
	Teams []TeamRecord
	Page  PageInfo
}

// MemberCandidateInput 定义可加入团队的成员查询条件。
type MemberCandidateInput struct {
	Query    string
	Page     int
	PageSize int
}

// MemberCandidate 定义可加入团队的成员字段。
type MemberCandidate struct {
	IdentityType domain.MemberIdentityType `bun:"identity_type"`
	IdentityID   string                    `bun:"identity_id"`
	DisplayName  string                    `bun:"display_name"`
	AvatarFileID *string                   `bun:"avatar_file_id"`
	RoleID       string                    `bun:"role_id"`
	RoleKind     domain.RoleKind           `bun:"role_kind"`
	RoleName     string                    `bun:"role_name"`
}

// MemberCandidateOutput 定义可加入团队的成员分页结果。
type MemberCandidateOutput struct {
	Members []MemberCandidate
	Page    PageInfo
}

// MemberIdentity 定义要加入团队的身份。
type MemberIdentity struct {
	Type domain.MemberIdentityType
	ID   string
}
