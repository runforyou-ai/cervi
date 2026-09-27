//go:build server

package integrationtest

import (
	"context"
	"strings"
	"testing"
	"uuid"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// testPublicURL 是集成测试使用的部署地址，服务端生成的对外链接以它为根地址。
const testPublicURL = "https://cervi.example.test"

// workspaceSpec 定义测试工作区与首位管理员账号；Email 在同一测试库内必须唯一，语言和时区为空时取中文与上海时区。
type workspaceSpec struct {
	Name        string
	DisplayName string
	Email       string
	Password    string
	Locale      domain.Locale
	TimeZone    string
}

// installedWorkspace 表示测试工作区中某个成员的身份及其账号登录令牌。
type installedWorkspace struct {
	Identity *servermodels.Identity
	Token    string
}

// installWorkspace 创建管理员账号和标识随机的工作区并签发登录会话；首次安装在每个部署只能执行一次，测试统一用它建立独立工作区。
func installWorkspace(t testing.TB, db *bun.DB, spec workspaceSpec) installedWorkspace {
	t.Helper()
	ctx := context.Background()
	if spec.Locale == "" {
		spec.Locale = domain.LocaleChineseSimplified
	}
	if spec.TimeZone == "" {
		spec.TimeZone = "Asia/Shanghai"
	}
	passwordHash, err := commonpassword.Hash(spec.Password)
	if err != nil {
		t.Fatal(err)
	}
	var token string
	var organizationID string
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		account, err := identityaction.CreateAccount(ctx, tx, identityaction.NewAccount{
			Email: spec.Email, PasswordHash: passwordHash, DisplayName: spec.DisplayName, Locale: spec.Locale, TimeZone: spec.TimeZone,
		})
		if err != nil {
			return err
		}
		created, err := organizationaction.Create(ctx, tx, organizationaction.CreateInput{
			Name: spec.Name, Slug: "ws-" + strings.ReplaceAll(uuid.NewV7().String(), "-", ""), Account: account, AdminDisplayName: spec.DisplayName,
		})
		if err != nil {
			return err
		}
		organizationID = created.Organization.ID
		session, err := authaction.IssueSession(ctx, tx, account)
		token = session.Token
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return resolveMemberSession(t, db, organizationID, token)
}

// loginMember 用账号密码登录，并解析该账号在目标工作区中的成员身份。
func loginMember(t testing.TB, db *bun.DB, organizationID, email, password string) installedWorkspace {
	t.Helper()
	session, err := authaction.NewLoginAction(db).Execute(context.Background(), authaction.LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatalf("login %s: %v", email, err)
	}
	return resolveMemberSession(t, db, organizationID, session.Token)
}

// resolveMemberSession 解析登录令牌在目标工作区中的成员身份。
func resolveMemberSession(t testing.TB, db *bun.DB, organizationID, token string) installedWorkspace {
	t.Helper()
	identity, err := authaction.NewResolveIdentityQuery(db).Execute(context.Background(), organizationID, token)
	if err != nil {
		t.Fatalf("resolve member session: %v", err)
	}
	return installedWorkspace{Identity: identity, Token: token}
}

// uniqueEmail 返回同一测试库内唯一的邮箱，local 用于标识测试中的角色。
func uniqueEmail(local string) string {
	return local + "." + strings.ReplaceAll(uuid.NewV7().String(), "-", "") + "@example.test"
}
