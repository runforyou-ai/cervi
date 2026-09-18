//go:build ios

#import <UserNotifications/UserNotifications.h>
#import "notification_ios.h"

// 查询状态和投递通知的最长等待时间。
static const int64_t cerviNotificationTimeoutNanos = 10 * NSEC_PER_SEC;

// 申请授权的最长等待时间，系统弹窗要等用户操作。
static const int64_t cerviNotificationAuthorizationTimeoutNanos = 2 * 60 * NSEC_PER_SEC;

// cerviNotificationStatus 把系统授权状态转换为对外状态取值。
static int cerviNotificationStatus(UNAuthorizationStatus status) {
    switch (status) {
        case UNAuthorizationStatusNotDetermined:
            return CERVI_NOTIFICATION_STATUS_PROMPT;
        case UNAuthorizationStatusDenied:
            return CERVI_NOTIFICATION_STATUS_DENIED;
        case UNAuthorizationStatusAuthorized:
        case UNAuthorizationStatusProvisional:
        case UNAuthorizationStatusEphemeral:
            return CERVI_NOTIFICATION_STATUS_GRANTED;
        default:
            return CERVI_NOTIFICATION_STATUS_UNKNOWN;
    }
}

// cerviNotificationString 把 C 字符串转换为非空 NSString。
static NSString *cerviNotificationString(const char *value) {
    if (value == NULL) {
        return @"";
    }
    NSString *converted = [NSString stringWithUTF8String:value];
    return converted != nil ? converted : @"";
}

int cervi_notification_authorization_status(void) {
    __block int status = CERVI_NOTIFICATION_STATUS_UNKNOWN;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    [[UNUserNotificationCenter currentNotificationCenter]
        getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
            status = cerviNotificationStatus(settings.authorizationStatus);
            dispatch_semaphore_signal(done);
        }];
    if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, cerviNotificationTimeoutNanos)) != 0) {
        return CERVI_NOTIFICATION_STATUS_UNKNOWN;
    }
    return status;
}

int cervi_notification_request_authorization(void) {
    __block int status = CERVI_NOTIFICATION_STATUS_UNKNOWN;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    UNAuthorizationOptions options =
        UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge;
    [[UNUserNotificationCenter currentNotificationCenter]
        requestAuthorizationWithOptions:options
                      completionHandler:^(BOOL granted, NSError *error) {
                          if (error != nil) {
                              status = CERVI_NOTIFICATION_STATUS_UNKNOWN;
                          } else {
                              status = granted ? CERVI_NOTIFICATION_STATUS_GRANTED
                                               : CERVI_NOTIFICATION_STATUS_DENIED;
                          }
                          dispatch_semaphore_signal(done);
                      }];
    if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, cerviNotificationAuthorizationTimeoutNanos)) != 0) {
        // 等待超时后按系统记录的授权状态返回，避免把用户已完成的授权报成失败。
        return cervi_notification_authorization_status();
    }
    return status;
}

int cervi_notification_post(const char *identifier, const char *title, const char *body, int silent) {
    UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
    content.title = cerviNotificationString(title);
    content.body = cerviNotificationString(body);
    if (silent == 0) {
        content.sound = [UNNotificationSound defaultSound];
    }
    NSString *requestID = cerviNotificationString(identifier);
    if (requestID.length == 0) {
        requestID = [[NSUUID UUID] UUIDString];
    }
    // trigger 为空表示立即投递，相同标识的通知在通知中心中替换前一条。
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:requestID
                                                                         content:content
                                                                         trigger:nil];
    __block int result = 0;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    [[UNUserNotificationCenter currentNotificationCenter]
        addNotificationRequest:request
         withCompletionHandler:^(NSError *error) {
             result = error != nil ? -1 : 0;
             dispatch_semaphore_signal(done);
         }];
    if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, cerviNotificationTimeoutNanos)) != 0) {
        return -1;
    }
    return result;
}
