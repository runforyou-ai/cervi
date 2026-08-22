//go:build server

package agent

import "github.com/runforyou-ai/cervi/internal/common"

const (
	ValidationDisplayNameRequired common.FieldCode = "DISPLAY_NAME_REQUIRED"
	ValidationTeamInvalid         common.FieldCode = "MEMBER_TEAM_INVALID"
	ValidationStatusInvalid       common.FieldCode = "USER_STATUS_INVALID"
)
