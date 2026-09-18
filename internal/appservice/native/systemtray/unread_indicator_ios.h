//go:build ios
// iOS 应用图标角标的原生入口。

#ifndef CERVI_UNREAD_INDICATOR_IOS_H
#define CERVI_UNREAD_INDICATOR_IOS_H

// cervi_unread_set_badge 设置应用图标角标数量，count 不大于 0 时清除角标。
void cervi_unread_set_badge(int count);

#endif
