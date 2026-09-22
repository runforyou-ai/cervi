//go:build server

package servicetimeout

import (
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestDueAction 验证提醒、回收与队列提醒的到期判断，AI 员工负责和没有等待的周期不参与。
func TestDueAction(t *testing.T) {
	now := time.Now()
	timeouts := domain.DefaultServiceTimeouts()
	ago := func(minutes int) *time.Time {
		value := now.Add(-time.Duration(minutes) * time.Minute)
		return &value
	}
	assignee := "assignee"
	open := string(domain.ServiceSessionStatusOpen)
	cases := []struct {
		name         string
		session      servermodels.ServiceSession
		assigneeType domain.OrganizationIdentityType
		want         timeoutAction
	}{
		{"未到提醒", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(4), AssigneeAssignedAt: ago(4)}, domain.OrganizationIdentityTypeUser, actionNone},
		{"提醒负责人", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(6), AssigneeAssignedAt: ago(6)}, domain.OrganizationIdentityTypeUser, actionRemindAssignee},
		{"已提醒", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(6), AssigneeAssignedAt: ago(6), RemindedAt: ago(1)}, domain.OrganizationIdentityTypeUser, actionNone},
		{"接手时间较晚", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(20), AssigneeAssignedAt: ago(2)}, domain.OrganizationIdentityTypeUser, actionNone},
		{"回收", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(16), AssigneeAssignedAt: ago(16), RemindedAt: ago(10)}, domain.OrganizationIdentityTypeUser, actionReclaim},
		{"AI 员工负责", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AwaitingReplySince: ago(30), AssigneeAssignedAt: ago(30)}, domain.OrganizationIdentityTypeAgent, actionNone},
		{"没有等待", servermodels.ServiceSession{Status: open, AssigneeIdentityID: &assignee, AssigneeAssignedAt: ago(30)}, domain.OrganizationIdentityTypeUser, actionNone},
		{"队列提醒", servermodels.ServiceSession{Status: open, AwaitingReplySince: ago(6)}, "", actionRemindQueue},
		{"队列已提醒", servermodels.ServiceSession{Status: open, AwaitingReplySince: ago(30), RemindedAt: ago(1)}, "", actionNone},
		{"已关闭", servermodels.ServiceSession{Status: string(domain.ServiceSessionStatusClosed), AwaitingReplySince: ago(30)}, "", actionNone},
	}
	for _, test := range cases {
		if got := dueAction(&test.session, test.assigneeType, timeouts, now); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
}
