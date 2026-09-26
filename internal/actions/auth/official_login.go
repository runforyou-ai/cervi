//go:build server

package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/officialidentity"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var (
	// ErrOfficialLoginInputInvalid 表示客户端提交的 state、nonce 或 PKCE 参数格式无效。
	ErrOfficialLoginInputInvalid = errors.New("official login input invalid")
	// ErrLoginAttemptInvalid 表示登录尝试不存在、已使用、已过期或 PKCE verifier 不匹配。
	ErrLoginAttemptInvalid = errors.New("login attempt invalid")
	// ErrOfficialAccountNotMember 表示官方账号在该企业没有有效的成员身份。
	ErrOfficialAccountNotMember = errors.New("official account is not an active member")
)

// officialLoginAttemptTTL 是登录尝试从发起到完成授权码交换的有效期。
const officialLoginAttemptTTL = 10 * time.Minute

// officialLoginCallbackPath 是企业域名下接收授权码的固定路径。
const officialLoginCallbackPath = "/auth/callback"

// officialLoginOpaquePattern 限定客户端生成的 state 与 nonce 为 16 到 128 位 URL 安全字符。
var officialLoginOpaquePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// pkcePattern 限定 PKCE verifier 与 S256 challenge 为 RFC 7636 规定的字符与长度。
var pkcePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

// OfficialIdentityProvider 定义官方账号登录使用的身份服务能力。
type OfficialIdentityProvider interface {
	Issuer() string
	AuthorizationURL(ctx context.Context, request officialidentity.AuthorizationRequest) (string, error)
	Exchange(ctx context.Context, request officialidentity.ExchangeRequest) (officialidentity.Claims, error)
}

// StartOfficialLoginAction 登记官方账号登录尝试并生成授权地址。
type StartOfficialLoginAction struct {
	db       *bun.DB
	provider OfficialIdentityProvider
}

// StartOfficialLoginInput 定义发起官方账号登录的输入，state、nonce 与 PKCE verifier 由客户端生成并保存。
type StartOfficialLoginInput struct {
	OrganizationID string
	AccessHost     string
	State          string
	Nonce          string
	CodeChallenge  string
}

// StartOfficialLoginOutput 返回登录尝试编号和授权地址。
type StartOfficialLoginOutput struct {
	AttemptID        string
	AuthorizationURL string
}

// NewStartOfficialLoginAction 创建发起官方账号登录操作。
func NewStartOfficialLoginAction(db *bun.DB, provider OfficialIdentityProvider) *StartOfficialLoginAction {
	return &StartOfficialLoginAction{db: db, provider: provider}
}

// Execute 生成授权地址后登记 Web 端登录尝试，回调地址固定为企业域名下的回调路径。
func (a *StartOfficialLoginAction) Execute(ctx context.Context, input StartOfficialLoginInput) (StartOfficialLoginOutput, error) {
	if !officialLoginOpaquePattern.MatchString(input.State) || !officialLoginOpaquePattern.MatchString(input.Nonce) || !pkcePattern.MatchString(input.CodeChallenge) || input.AccessHost == "" {
		return StartOfficialLoginOutput{}, ErrOfficialLoginInputInvalid
	}
	redirectURI := "https://" + input.AccessHost + officialLoginCallbackPath
	authorizationURL, err := a.provider.AuthorizationURL(ctx, officialidentity.AuthorizationRequest{
		RedirectURI:   redirectURI,
		State:         input.State,
		Nonce:         input.Nonce,
		CodeChallenge: input.CodeChallenge,
	})
	if err != nil {
		return StartOfficialLoginOutput{}, err
	}
	attempt := &servermodels.LoginAttempt{
		OrganizationID: input.OrganizationID,
		Purpose:        string(domain.LoginAttemptPurposeLogin),
		ClientType:     string(domain.OfficialLoginClientWeb),
		RedirectURI:    redirectURI,
		CodeChallenge:  input.CodeChallenge,
		Nonce:          input.Nonce,
		ExpiresAt:      time.Now().Add(officialLoginAttemptTTL),
	}
	if _, err := a.db.NewInsert().Model(attempt).
		Column("organization_id", "purpose", "client_type", "redirect_uri", "code_challenge", "nonce", "expires_at").
		Returning("id::text").
		Exec(ctx); err != nil {
		return StartOfficialLoginOutput{}, fmt.Errorf("save login attempt: %w", err)
	}
	return StartOfficialLoginOutput{AttemptID: attempt.ID, AuthorizationURL: authorizationURL}, nil
}

// CompleteOfficialLoginAction 用授权码完成官方账号登录并签发登录令牌。
type CompleteOfficialLoginAction struct {
	db       *bun.DB
	provider OfficialIdentityProvider
}

// CompleteOfficialLoginInput 定义完成官方账号登录的输入。
type CompleteOfficialLoginInput struct {
	OrganizationID string
	AttemptID      string
	Code           string
	CodeVerifier   string
}

// NewCompleteOfficialLoginAction 创建完成官方账号登录操作。
func NewCompleteOfficialLoginAction(db *bun.DB, provider OfficialIdentityProvider) *CompleteOfficialLoginAction {
	return &CompleteOfficialLoginAction{db: db, provider: provider}
}

// Execute 一次性消费登录尝试，交换授权码并校验 ID Token，再按外部身份绑定签发该企业成员的登录令牌。
func (a *CompleteOfficialLoginAction) Execute(ctx context.Context, input CompleteOfficialLoginInput) (LoginOutput, error) {
	if input.Code == "" || !pkcePattern.MatchString(input.CodeVerifier) {
		return LoginOutput{}, ErrLoginAttemptInvalid
	}
	attempt, err := a.consumeAttempt(ctx, input)
	if err != nil {
		return LoginOutput{}, err
	}
	claims, err := a.provider.Exchange(ctx, officialidentity.ExchangeRequest{
		Code:         input.Code,
		RedirectURI:  attempt.RedirectURI,
		CodeVerifier: input.CodeVerifier,
		Nonce:        attempt.Nonce,
	})
	if err != nil {
		return LoginOutput{}, err
	}

	var output LoginOutput
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		// 按可信 issuer 与稳定 subject 找到该企业内绑定的在职成员。
		var userID string
		err := tx.NewSelect().
			TableExpr("external_identities AS ei").
			ColumnExpr("u.id::text").
			Join("JOIN users AS u ON u.id = ei.user_id AND u.organization_id = ei.organization_id").
			Join("JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeUser).
			Where("ei.organization_id = ?", input.OrganizationID).
			Where("ei.issuer = ?", a.provider.Issuer()).
			Where("ei.subject = ?", claims.Subject).
			Where("u.status = ?", domain.UserStatusActive).
			Scan(ctx, &userID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOfficialAccountNotMember
		}
		if err != nil {
			return fmt.Errorf("find official account member: %w", err)
		}
		issued, identity, err := issueToken(ctx, tx, input.OrganizationID, userID)
		if err != nil {
			return err
		}
		output = LoginOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}
		return nil
	})
	if err != nil {
		return LoginOutput{}, err
	}
	return output, nil
}

// consumeAttempt 锁定并消费本企业仍有效的 Web 登录尝试，并校验 PKCE verifier 与登记的 challenge 一致。
func (a *CompleteOfficialLoginAction) consumeAttempt(ctx context.Context, input CompleteOfficialLoginInput) (*servermodels.LoginAttempt, error) {
	attempt := &servermodels.LoginAttempt{}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := tx.NewSelect().Model(attempt).
			Where("la.id::text = ?", input.AttemptID).
			Where("la.organization_id = ?", input.OrganizationID).
			Where("la.purpose = ?", domain.LoginAttemptPurposeLogin).
			Where("la.client_type = ?", domain.OfficialLoginClientWeb).
			Where("la.consumed_at IS NULL").
			Where("la.expires_at > now()").
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLoginAttemptInvalid
		}
		if err != nil {
			return fmt.Errorf("find login attempt: %w", err)
		}
		if _, err := tx.NewUpdate().Model(attempt).
			Set("consumed_at = now()").
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("consume login attempt: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// S256 challenge 为 verifier 的 SHA-256 摘要的 base64url 编码（无填充）。
	digest := sha256.Sum256([]byte(input.CodeVerifier))
	if base64.RawURLEncoding.EncodeToString(digest[:]) != attempt.CodeChallenge {
		return nil, ErrLoginAttemptInvalid
	}
	return attempt, nil
}
