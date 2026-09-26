//go:build server

package integrationtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/officialidentity"
	"github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

const (
	testOfficialWebClientID     = "cervi-web"
	testOfficialWebClientSecret = "cervi-web-secret"
	testOfficialCodeVerifier    = "official-login-code-verifier-0123456789abcdefghijklmnop"
)

// fakeIdentityProvider 是签发 RS256 ID Token 的测试用 OIDC 提供方，令牌中的 subject 与 nonce 由测试指定。
type fakeIdentityProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey

	mu      sync.Mutex
	subject string
	nonce   string
}

// newFakeIdentityProvider 启动提供发现文档、JWKS 与令牌端点的 HTTPS 测试服务。
func newFakeIdentityProvider(t *testing.T) *fakeIdentityProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	provider := &fakeIdentityProvider{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(writer http.ResponseWriter, request *http.Request) {
		issuer := provider.server.URL
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/oauth/authorize",
			"token_endpoint":                        issuer + "/oauth/token",
			"jwks_uri":                              issuer + "/oauth/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/oauth/jwks", func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/oauth/token", func(writer http.ResponseWriter, request *http.Request) {
		// 令牌端点校验客户端凭据与 PKCE verifier，模拟身份服务对授权码的校验。
		clientID, clientSecret, _ := request.BasicAuth()
		if request.ParseForm() != nil || clientID != testOfficialWebClientID || clientSecret != testOfficialWebClientSecret ||
			request.PostForm.Get("code") != "valid-code" || request.PostForm.Get("code_verifier") != testOfficialCodeVerifier {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		provider.mu.Lock()
		claims := jwt.MapClaims{
			"iss": provider.server.URL, "sub": provider.subject, "aud": testOfficialWebClientID,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "nonce": provider.nonce,
			"email": "member@official.test", "email_verified": true, "name": "官方成员",
		}
		provider.mu.Unlock()
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test-key"
		signed, err := token.SignedString(key)
		if err != nil {
			t.Error(err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "access", "token_type": "Bearer", "expires_in": 600, "id_token": signed})
	})
	provider.server = httptest.NewTLSServer(mux)
	t.Cleanup(provider.server.Close)
	return provider
}

// issue 指定下一次签发的 ID Token 中的 subject 与 nonce。
func (p *fakeIdentityProvider) issue(subject string, nonce string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subject, p.nonce = subject, nonce
}

// officialLoginFixture 是官方账号登录测试使用的托管企业、成员与后端。
type officialLoginFixture struct {
	db       *bun.DB
	provider *fakeIdentityProvider
	backend  *appservice.DirectBackend
	ctx      context.Context
	subject  string
	userID   string
}

// newOfficialLoginFixture 通过运营开通创建绑定官方身份的托管企业，并创建使用测试身份服务的托管后端。
func newOfficialLoginFixture(t *testing.T) officialLoginFixture {
	t.Helper()
	provider := newFakeIdentityProvider(t)
	store, err := serverstorage.Open(context.Background(), servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db := store.DB()
	operator := appservice.NewOperatorDirectBackend(db, appservice.OperatorConfig{
		Deployment:             appservice.OperatorDeployment{Mode: appservice.DeploymentModeManaged, ManagedDomainSuffix: testProvisioningSuffix},
		Credential:             testProvisioningCredential,
		OfficialIdentityIssuer: provider.server.URL,
	})
	input := newProvisionInput()
	provisioned, err := operator.ProvisionOrganization(context.Background(), appservice.OperatorRequestMeta{Credential: testProvisioningCredential, RequestID: "official-login-test"}, input)
	if err != nil {
		t.Fatal(err)
	}
	client := officialidentity.NewClient(officialidentity.Config{
		Issuer: provider.server.URL, WebClientID: testOfficialWebClientID, WebClientSecret: testOfficialWebClientSecret,
		HTTPClient: provider.server.Client(),
	})
	backend := appservice.NewDirectBackend(db, appservice.DirectDeploymentConfig{Mode: domain.DeploymentModeManaged, OfficialIdentity: client},
		nil, serverfilecontent.S3Config{}, serverstorage.NewTenantResolver(db), nil, nil, nil, nil, nil)
	return officialLoginFixture{
		db: db, provider: provider, backend: backend,
		ctx:     tenant.WithAccessHost(context.Background(), provisioned.AccessHost),
		subject: input.InitialUser.Subject, userID: provisioned.InitialUserID,
	}
}

// start 发起一次登录尝试，返回尝试编号和授权地址中的参数。
func (f officialLoginFixture) start(t *testing.T, nonce string) (string, url.Values) {
	t.Helper()
	digest := sha256.Sum256([]byte(testOfficialCodeVerifier))
	started, err := f.backend.StartOfficialLogin(f.ctx, appservice.RequestMeta{}, appservice.OfficialLoginInput{
		State: "state-0123456789abcdef", Nonce: nonce, CodeChallenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	return started.AttemptID, authorizationURL.Query()
}

// complete 用测试授权码完成登录尝试。
func (f officialLoginFixture) complete(attemptID string, verifier string) (appservice.Auth, error) {
	return f.backend.CompleteOfficialLogin(f.ctx, appservice.RequestMeta{}, appservice.OfficialLoginCompletion{AttemptID: attemptID, Code: "valid-code", CodeVerifier: verifier})
}

// requireErrorKey 断言错误是使用指定文案键的业务错误。
func requireErrorKey(t *testing.T, err error, key cervii18n.Key) {
	t.Helper()
	expected, _ := cervii18n.Localize("", key)
	var appError *appservice.Error
	if !errors.As(err, &appError) || appError.Message != expected {
		t.Fatalf("错误应为 %s（%s），实际为 %v", key, expected, err)
	}
}

// TestOfficialLoginSignsInBoundMember 验证官方账号登录签发绑定成员的企业令牌，授权地址携带 PKCE、nonce 与企业回调地址。
func TestOfficialLoginSignsInBoundMember(t *testing.T) {
	f := newOfficialLoginFixture(t)
	nonce := "nonce-0123456789abcdef"
	attemptID, query := f.start(t, nonce)

	if query.Get("client_id") != testOfficialWebClientID || query.Get("code_challenge_method") != "S256" || query.Get("nonce") != nonce ||
		query.Get("state") != "state-0123456789abcdef" || !strings.HasSuffix(query.Get("redirect_uri"), "/auth/callback") ||
		!strings.HasPrefix(query.Get("redirect_uri"), "https://") || !strings.Contains(query.Get("scope"), "openid") {
		t.Fatalf("授权地址参数不正确: %v", query)
	}

	f.provider.issue(f.subject, nonce)
	auth, err := f.complete(attemptID, testOfficialCodeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Token == "" || auth.Identity.User.ID != f.userID {
		t.Fatalf("登录结果不正确: %+v", auth.Identity.User)
	}
	identity, err := f.backend.LoadIdentity(f.ctx, appservice.RequestMeta{Token: auth.Token})
	if err != nil || identity.User.ID != f.userID {
		t.Fatalf("签发的令牌不可用: %v", err)
	}

	// 登录尝试只能使用一次。
	_, err = f.complete(attemptID, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginExpired)
}

// TestOfficialLoginRejectsInvalidAttempts 验证 verifier 不匹配、过期或属于其他企业的登录尝试被拒绝。
func TestOfficialLoginRejectsInvalidAttempts(t *testing.T) {
	f := newOfficialLoginFixture(t)
	nonce := "nonce-0123456789abcdef"
	f.provider.issue(f.subject, nonce)

	wrongVerifier, _ := f.start(t, nonce)
	_, err := f.complete(wrongVerifier, strings.Repeat("x", 43))
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginExpired)

	expired, _ := f.start(t, nonce)
	if _, err := f.db.NewUpdate().Table("login_attempts").Set("expires_at = now() - interval '1 second'").Where("id = ?", expired).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = f.complete(expired, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginExpired)

	other := newOfficialLoginFixture(t)
	foreign, _ := other.start(t, nonce)
	_, err = f.complete(foreign, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginExpired)
}

// TestOfficialLoginRejectsUntrustedIdentity 验证 nonce 不一致、未绑定的官方账号和已停用成员不能登录。
func TestOfficialLoginRejectsUntrustedIdentity(t *testing.T) {
	f := newOfficialLoginFixture(t)
	nonce := "nonce-0123456789abcdef"

	attemptID, _ := f.start(t, nonce)
	f.provider.issue(f.subject, "nonce-from-another-request")
	_, err := f.complete(attemptID, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginRejected)

	attemptID, _ = f.start(t, nonce)
	f.provider.issue("subject-without-binding", nonce)
	_, err = f.complete(attemptID, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialAccountNotMember)

	if _, err := f.db.NewUpdate().Table("users").Set("status = ?", domain.UserStatusInactive).Where("id = ?", f.userID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	attemptID, _ = f.start(t, nonce)
	f.provider.issue(f.subject, nonce)
	_, err = f.complete(attemptID, testOfficialCodeVerifier)
	requireErrorKey(t, err, cervii18n.ErrorOfficialAccountNotMember)
}

// TestOfficialLoginAvailability 验证自托管部署不提供官方账号登录，身份服务不可用时返回对应错误。
func TestOfficialLoginAvailability(t *testing.T) {
	f := newOfficialLoginFixture(t)
	selfHosted := appservice.NewDirectBackend(f.db, appservice.DirectDeploymentConfig{Mode: domain.DeploymentModeSelfHosted},
		nil, serverfilecontent.S3Config{}, serverstorage.NewTenantResolver(f.db), nil, nil, nil, nil, nil)
	_, err := selfHosted.StartOfficialLogin(f.ctx, appservice.RequestMeta{}, appservice.OfficialLoginInput{})
	requireErrorKey(t, err, cervii18n.ErrorOfficialLoginNotAvailable)

	f.provider.server.Close()
	digest := sha256.Sum256([]byte(testOfficialCodeVerifier))
	_, err = f.backend.StartOfficialLogin(f.ctx, appservice.RequestMeta{}, appservice.OfficialLoginInput{
		State: "state-0123456789abcdef", Nonce: "nonce-0123456789abcdef", CodeChallenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	})
	requireErrorKey(t, err, cervii18n.ErrorOfficialIdentityUnavailable)
}
