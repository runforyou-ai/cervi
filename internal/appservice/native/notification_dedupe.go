//go:build !server

package native

import (
	"sync"
	"time"
)

// notificationDedupeWindow 是同一通知编号只投递一次的时长。
const notificationDedupeWindow = 10 * time.Minute

// deliveredNotifications 记录近期投递过的通知编号，同一进程内多处观察到同一条消息时只投递一次。
type deliveredNotifications struct {
	mu        sync.Mutex
	delivered map[string]time.Time
}

// firstDelivery 登记通知编号，编号在去重时长内已投递过时返回 false；空编号不去重。
func (d *deliveredNotifications) firstDelivery(id string) bool {
	if id == "" {
		return true
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.delivered == nil {
		d.delivered = map[string]time.Time{}
	}
	for previous, at := range d.delivered {
		if now.Sub(at) > notificationDedupeWindow {
			delete(d.delivered, previous)
		}
	}
	if _, ok := d.delivered[id]; ok {
		return false
	}
	d.delivered[id] = now
	return true
}
