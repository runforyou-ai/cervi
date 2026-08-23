//go:build server

package user

import (
	"context"
	"errors"
	"testing"
)

// TestListUsersRejectsInvalidFilters 验证账号状态和角色编号。
func TestListUsersRejectsInvalidFilters(t *testing.T) {
	query := NewListUsersQuery(nil)
	for _, input := range []ListInput{
		{Status: "deleted", Page: 1, PageSize: 50},
		{RoleID: "administrator", Page: 1, PageSize: 50},
		{Page: 1, PageSize: 101},
	} {
		if _, err := query.Execute(context.Background(), nil, input); !errors.Is(err, ErrQueryInvalid) {
			t.Fatalf("query error = %v, want %v", err, ErrQueryInvalid)
		}
	}
}
