//go:build ios
// Minimal bootstrap: delegate comes from Go archive (WailsAppDelegate)
#import <UIKit/UIKit.h>
#include <stdio.h>

int main(int argc, char * argv[]) {
    @autoreleasepool {
        // Disable buffering so stdout/stderr from Go log.Printf flush immediately
        setvbuf(stdout, NULL, _IONBF, 0);
        setvbuf(stderr, NULL, _IONBF, 0);

        // 先启动 UIKit，Go 运行时由 WailsAppDelegate 的 didFinishLaunchingWithOptions 启动。
        return UIApplicationMain(argc, argv, nil, @"WailsAppDelegate");
    }
}
