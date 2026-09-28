//go:build !server

package native

import "testing"

// TestDeliveredNotificationsOncePerID 验证同一通知编号在去重时长内只投递一次，不同编号与空编号照常投递。
func TestDeliveredNotificationsOncePerID(t *testing.T) {
	var delivered deliveredNotifications
	if !delivered.firstDelivery("m1") || delivered.firstDelivery("m1") {
		t.Fatal("同一编号应只投递一次")
	}
	if !delivered.firstDelivery("m2") || !delivered.firstDelivery("") || !delivered.firstDelivery("") {
		t.Fatal("不同编号与空编号应照常投递")
	}
}
