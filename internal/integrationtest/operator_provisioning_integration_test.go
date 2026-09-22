//go:build server

package integrationtest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	testProvisioningSuffix     = "provisioning.test"
	testProvisioningIssuer     = "https://account.provisioning.test"
	testProvisioningCredential = "operator-credential-operator-credential"
)

// provisioningFixture 是运营开通测试使用的数据库和运营后端。
type provisioningFixture struct {
	db      *bun.DB
	backend *appservice.OperatorDirectBackend
	meta    appservice.OperatorRequestMeta
}

// newProvisioningFixture 打开测试数据库并创建托管部署的运营后端。
func newProvisioningFixture(t *testing.T) provisioningFixture {
	t.Helper()
	store, err := serverstorage.Open(context.Background(), servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	backend := appservice.NewOperatorDirectBackend(store.DB(), appservice.OperatorConfig{
		Deployment: appservice.OperatorDeployment{
			Mode:                appservice.DeploymentModeManaged,
			ManagedDomainSuffix: testProvisioningSuffix,
		},
		Credential:             testProvisioningCredential,
		OfficialIdentityIssuer: testProvisioningIssuer,
	})
	return provisioningFixture{
		db:      store.DB(),
		backend: backend,
		meta:    appservice.OperatorRequestMeta{Credential: testProvisioningCredential, RequestID: "provisioning-test"},
	}
}

// newProvisionInput 生成使用唯一开通标识、前缀和官方 subject 的开通请求。
func newProvisionInput() appservice.OperatorProvisionInput {
	unique := strings.ReplaceAll(uuid.NewV7().String(), "-", "")
	return appservice.OperatorProvisionInput{
		ProvisioningID:   "prov-" + unique,
		OrganizationName: "托管企业",
		DomainPrefix:     "T" + unique,
		InitialUser: appservice.OperatorInitialUserInput{
			Subject:     "subject-" + unique,
			DisplayName: "企业创建者",
			Email:       "owner@" + unique + ".provisioning.test",
			Locale:      appservice.LocaleChineseSimplified,
			TimeZone:    "Asia/Shanghai",
		},
		Entitlement: appservice.OperatorEntitlementInput{Revision: 1, PlanCode: "trial"},
	}
}

// requireOperatorCode 断言错误是带指定错误码的运营错误。
func requireOperatorCode(t *testing.T, err error, code appservice.OperatorErrorCode) {
	t.Helper()
	operatorError, ok := appservice.OperatorErrorOf(err)
	if !ok || operatorError.Code != code {
		t.Fatalf("错误 = %v，期望错误码 %s", err, code)
	}
}

// TestOperatorProvisioningCreatesManagedOrganization 验证开通在同一事务内创建企业、无本地密码的初始成员、外部身份绑定和初始权益。
func TestOperatorProvisioningCreatesManagedOrganization(t *testing.T) {
	f := newProvisioningFixture(t)
	ctx := context.Background()
	input := newProvisionInput()
	endsAt := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	input.Entitlement.ServiceEndsAt = &endsAt

	result, err := f.backend.ProvisionOrganization(ctx, f.meta, input)
	if err != nil {
		t.Fatal(err)
	}
	wantHost := strings.ToLower(input.DomainPrefix) + "." + testProvisioningSuffix
	if result.AccessHost != wantHost || result.PublicURL != "https://"+wantHost ||
		result.LifecycleStatus != appservice.OrganizationLifecycleActive || result.EntitlementRevision != 1 {
		t.Fatalf("开通结果 = %+v", result)
	}

	binding := servermodels.ExternalIdentity{}
	if err := f.db.NewSelect().Model(&binding).Where("ei.user_id = ?", result.InitialUserID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if binding.OrganizationID != result.OrganizationID || binding.Issuer != testProvisioningIssuer || binding.Subject != input.InitialUser.Subject {
		t.Fatalf("外部身份绑定 = %+v", binding)
	}
	var passwordMissing bool
	if err := f.db.NewSelect().TableExpr("users").ColumnExpr("password_hash IS NULL").
		Where("id = ?", result.InitialUserID).Scan(ctx, &passwordMissing); err != nil {
		t.Fatal(err)
	}
	if !passwordMissing {
		t.Fatal("托管初始成员带有本地密码")
	}
	if _, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{
		OrganizationID: result.OrganizationID, Email: input.InitialUser.Email, Password: "password123",
	}); !errors.Is(err, authaction.ErrInvalidCredentials) {
		t.Fatalf("托管成员本地密码登录结果 = %v", err)
	}

	stored, err := f.backend.GetProvisioning(ctx, f.meta, input.ProvisioningID)
	if err != nil || stored != result {
		t.Fatalf("开通结果查询 = %+v, %v", stored, err)
	}
	organization, err := f.backend.GetOrganization(ctx, f.meta, result.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if organization.ProvisioningID == nil || *organization.ProvisioningID != input.ProvisioningID || organization.Entitlement == nil ||
		!organization.Entitlement.Valid || organization.Entitlement.PlanCode != "trial" || !organization.Entitlement.ServiceEndsAt.Equal(endsAt) {
		t.Fatalf("企业摘要 = %+v", organization)
	}

	taken, err := f.backend.CheckDomainAvailability(ctx, f.meta, appservice.OperatorDomainAvailabilityInput{Prefix: input.DomainPrefix})
	if err != nil || taken.Available || taken.AccessHost != wantHost {
		t.Fatalf("已占用前缀的可用性 = %+v, %v", taken, err)
	}
	free, err := f.backend.CheckDomainAvailability(ctx, f.meta, appservice.OperatorDomainAvailabilityInput{Prefix: newProvisionInput().DomainPrefix})
	if err != nil || !free.Available {
		t.Fatalf("未占用前缀的可用性 = %+v, %v", free, err)
	}

	list, err := f.backend.ListOrganizations(ctx, f.meta, appservice.OperatorOrganizationListInput{Query: wantHost, Page: 1, PageSize: 10})
	if err != nil || list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != result.OrganizationID {
		t.Fatalf("按访问地址筛选的企业列表 = %+v, %v", list, err)
	}
}

// TestOperatorProvisioningIsIdempotent 验证同一开通标识重复提交返回同一企业，输入变化返回冲突。
func TestOperatorProvisioningIsIdempotent(t *testing.T) {
	f := newProvisioningFixture(t)
	ctx := context.Background()
	input := newProvisionInput()

	first, err := f.backend.ProvisionOrganization(ctx, f.meta, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.backend.ProvisionOrganization(ctx, f.meta, input)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("重复开通结果不同：%+v 与 %+v", first, second)
	}

	changed := input
	changed.OrganizationName = "另一个名称"
	_, err = f.backend.ProvisionOrganization(ctx, f.meta, changed)
	requireOperatorCode(t, err, appservice.OperatorErrorCodeIdempotencyConflict)

	// 官方身份 subject 逐字符比较，首尾空白不同即视为不同身份。
	padded := input
	padded.InitialUser.Subject = " " + input.InitialUser.Subject
	_, err = f.backend.ProvisionOrganization(ctx, f.meta, padded)
	requireOperatorCode(t, err, appservice.OperatorErrorCodeIdempotencyConflict)

	_, err = f.backend.GetProvisioning(ctx, f.meta, "prov-missing-"+uuid.NewV7().String())
	requireOperatorCode(t, err, appservice.OperatorErrorCodeProvisioningNotFound)
	_, err = f.backend.GetOrganization(ctx, f.meta, uuid.NewV7().String())
	requireOperatorCode(t, err, appservice.OperatorErrorCodeOrganizationNotFound)
}

// TestOperatorProvisioningRejectsTakenDomain 验证不同开通标识抢占已登记域名时返回域名已占用。
func TestOperatorProvisioningRejectsTakenDomain(t *testing.T) {
	f := newProvisioningFixture(t)
	ctx := context.Background()
	input := newProvisionInput()
	if _, err := f.backend.ProvisionOrganization(ctx, f.meta, input); err != nil {
		t.Fatal(err)
	}
	other := newProvisionInput()
	other.DomainPrefix = strings.ToLower(input.DomainPrefix)
	_, err := f.backend.ProvisionOrganization(ctx, f.meta, other)
	requireOperatorCode(t, err, appservice.OperatorErrorCodeDomainTaken)

	invalid := newProvisionInput()
	invalid.DomainPrefix = "acme_corp"
	_, err = f.backend.ProvisionOrganization(ctx, f.meta, invalid)
	requireOperatorCode(t, err, appservice.OperatorErrorCodeInvalidDomainPrefix)
}

// TestOperatorProvisioningRejectsInvalidFields 验证空白显示名和含路径分隔符的开通标识被拒绝且不创建企业。
func TestOperatorProvisioningRejectsInvalidFields(t *testing.T) {
	f := newProvisioningFixture(t)
	ctx := context.Background()
	for field, mutate := range map[string]func(*appservice.OperatorProvisionInput){
		"initialUser.displayName": func(input *appservice.OperatorProvisionInput) { input.InitialUser.DisplayName = "   " },
		"provisioningId":          func(input *appservice.OperatorProvisionInput) { input.ProvisioningID = "prov/" + uuid.NewV7().String() },
	} {
		input := newProvisionInput()
		mutate(&input)
		_, err := f.backend.ProvisionOrganization(ctx, f.meta, input)
		operatorError, ok := appservice.OperatorErrorOf(err)
		if !ok || operatorError.Code != appservice.OperatorErrorCodeInvalidRequest || operatorError.Fields[field] == "" {
			t.Fatalf("%s 校验结果 = %v", field, err)
		}
		availability, err := f.backend.CheckDomainAvailability(ctx, f.meta, appservice.OperatorDomainAvailabilityInput{Prefix: input.DomainPrefix})
		if err != nil || !availability.Available {
			t.Fatalf("%s 校验失败后域名被占用：%+v, %v", field, availability, err)
		}
	}
}

// TestOperatorProvisioningConcurrentRequests 验证并发抢占同一域名只有一个成功归属，并发重试同一开通标识得到同一企业。
func TestOperatorProvisioningConcurrentRequests(t *testing.T) {
	f := newProvisioningFixture(t)
	ctx := context.Background()
	const workers = 6

	shared := newProvisionInput()
	var wait sync.WaitGroup
	results := make([]appservice.OperatorProvisioning, workers)
	errs := make([]error, workers)
	for index := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			input := newProvisionInput()
			input.DomainPrefix = shared.DomainPrefix
			results[index], errs[index] = f.backend.ProvisionOrganization(ctx, f.meta, input)
		}()
	}
	wait.Wait()
	succeeded := 0
	for index, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		requireOperatorCode(t, errs[index], appservice.OperatorErrorCodeDomainTaken)
	}
	if succeeded != 1 {
		t.Fatalf("并发抢占同一域名成功 %d 次", succeeded)
	}

	retried := newProvisionInput()
	for index := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index], errs[index] = f.backend.ProvisionOrganization(ctx, f.meta, retried)
		}()
	}
	wait.Wait()
	for index := range workers {
		if errs[index] != nil {
			t.Fatalf("并发重试同一开通标识失败：%v", errs[index])
		}
		if results[index] != results[0] {
			t.Fatalf("并发重试得到不同企业：%+v 与 %+v", results[index], results[0])
		}
	}
}
