//go:build server

package integrationtest

import (
	"context"
	"reflect"
	"testing"
	"uuid"

	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
)

// publicBackendMethods 是企业初始化前即可调用、不解析登录身份的方法，
// 与 backend.go 中标记 auth=public 的路由一一对应。
var publicBackendMethods = map[string]bool{
	"InstallationStatus": true,
	"Login":              true,
}

// TestBackendMethodsRequireAuthentication 验证除公开方法外，
// 每个 Backend 方法在无令牌时都被挡回登录入口。
//
// member 方法漏掉认证通常先编译失败，因为业务实现要求 identity。
// 本测试锁定编译期覆盖不到的三种情况：生成器认证分发回归、
// 方法被误标 auth=public、以及 sessionGuard 的运行时接线断开。
func TestBackendMethodsRequireAuthentication(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db := store.DB()

	// 已初始化的企业让请求越过安装校验，停在令牌校验上。
	accessHost := uuid.NewV7().String() + ".authentication.test"
	if _, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
		AccessHost: accessHost, OrganizationName: "认证测试", DisplayName: "管理员",
		Email: "admin@authentication.test", Password: "password123",
		Locale: domain.LocaleEnglishUnitedStates, TimeZone: "UTC",
	}); err != nil {
		t.Fatal(err)
	}

	backend := appservice.NewDirectBackend(db, nil, serverstorage.NewTenantResolver(db), nil, nil, nil, nil)
	tenantContext := tenant.WithAccessHost(ctx, accessHost)
	backendValue := reflect.ValueOf(backend)
	backendInterface := reflect.TypeFor[appservice.Backend]()

	checked := 0
	for index := range backendInterface.NumMethod() {
		name := backendInterface.Method(index).Name
		if publicBackendMethods[name] {
			continue
		}
		method := backendValue.MethodByName(name)
		// 认证发生在业务实现之前，其余参数取零值即可到达令牌校验。
		arguments := make([]reflect.Value, method.Type().NumIn())
		arguments[0] = reflect.ValueOf(tenantContext)
		arguments[1] = reflect.ValueOf(appservice.RequestMeta{})
		for argument := 2; argument < len(arguments); argument++ {
			arguments[argument] = reflect.New(method.Type().In(argument)).Elem()
		}
		results := method.Call(arguments)
		err, _ := results[len(results)-1].Interface().(error)
		if state := appservice.SessionStateOf(err); state != appservice.SessionStateLogin {
			t.Errorf("%s 未认证调用的会话入口 = %q, want %q（错误：%v）", name, state, appservice.SessionStateLogin, err)
		}
		checked++
	}
	// 公开名单与接口闭合，防止名字写错或过期后静默跳过方法。
	for name := range publicBackendMethods {
		if _, exists := backendInterface.MethodByName(name); !exists {
			t.Errorf("公开方法名单中的 %s 已不在 Backend 接口上", name)
		}
	}
	if total := checked + len(publicBackendMethods); total != backendInterface.NumMethod() {
		t.Fatalf("已校验 %d 个方法加 %d 个公开方法 = %d，Backend 接口共 %d 个方法",
			checked, len(publicBackendMethods), total, backendInterface.NumMethod())
	}
	t.Logf("已验证 %d 个 Backend 方法要求认证", checked)
}
