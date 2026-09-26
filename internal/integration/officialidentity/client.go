// Package officialidentity 对接官方身份服务（OIDC 提供方），生成授权地址并完成授权码交换与 ID Token 校验。
package officialidentity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	// ErrUnavailable 表示官方身份服务暂时无法访问。
	ErrUnavailable = errors.New("official identity service unavailable")
	// ErrRejected 表示官方身份服务拒绝了授权码，或返回的 ID Token 未通过校验。
	ErrRejected = errors.New("official identity rejected")
)

// Config 定义可信官方身份服务及 Web 客户端凭据。
type Config struct {
	Issuer          string
	WebClientID     string
	WebClientSecret string
	HTTPClient      *http.Client
}

// Claims 是 ID Token 中本服务使用的身份声明。
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// AuthorizationRequest 定义生成授权地址所需的参数。
type AuthorizationRequest struct {
	RedirectURI   string
	State         string
	Nonce         string
	CodeChallenge string
}

// ExchangeRequest 定义授权码交换及 ID Token 校验所需的参数。
type ExchangeRequest struct {
	Code         string
	RedirectURI  string
	CodeVerifier string
	Nonce        string
}

// Client 访问官方身份服务；发现文档在首次使用时读取并缓存。
type Client struct {
	config   Config
	mu       sync.Mutex
	provider *oidc.Provider
}

// NewClient 创建官方身份服务客户端，未指定 HTTP 客户端时使用 10 秒超时。
func NewClient(config Config) *Client {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{config: config}
}

// Issuer 返回可信官方身份服务标识。
func (c *Client) Issuer() string {
	return c.config.Issuer
}

// AuthorizationURL 返回带 PKCE S256 challenge 与 nonce 的授权地址。
func (c *Client) AuthorizationURL(ctx context.Context, request AuthorizationRequest) (string, error) {
	provider, err := c.discover(ctx)
	if err != nil {
		return "", err
	}
	return c.oauth2Config(provider, request.RedirectURI).AuthCodeURL(
		request.State,
		oidc.Nonce(request.Nonce),
		oauth2.SetAuthURLParam("code_challenge", request.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

// Exchange 用授权码换取令牌，校验 ID Token 的签名、iss、aud、exp 与 nonce 后返回身份声明。
func (c *Client) Exchange(ctx context.Context, request ExchangeRequest) (Claims, error) {
	provider, err := c.discover(ctx)
	if err != nil {
		return Claims{}, err
	}
	ctx = oidc.ClientContext(ctx, c.config.HTTPClient)
	token, err := c.oauth2Config(provider, request.RedirectURI).Exchange(ctx, request.Code, oauth2.VerifierOption(request.CodeVerifier))
	if err != nil {
		// 身份服务返回错误响应表示授权码或 verifier 无效，其余错误视为服务不可用。
		if _, rejected := errors.AsType[*oauth2.RetrieveError](err); rejected {
			return Claims{}, fmt.Errorf("%w: %v", ErrRejected, err)
		}
		return Claims{}, fmt.Errorf("%w: exchange code: %v", ErrUnavailable, err)
	}
	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return Claims{}, fmt.Errorf("%w: missing id_token", ErrRejected)
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: c.config.WebClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: verify id_token: %v", ErrRejected, err)
	}
	if idToken.Nonce != request.Nonce {
		return Claims{}, fmt.Errorf("%w: nonce mismatch", ErrRejected)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Claims{}, fmt.Errorf("%w: decode claims: %v", ErrRejected, err)
	}
	return Claims{Subject: idToken.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified, Name: claims.Name}, nil
}

// discover 读取并缓存发现文档，读取失败时不缓存。
func (c *Client) discover(ctx context.Context) (*oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider != nil {
		return c.provider, nil
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, c.config.HTTPClient), c.config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: discover: %v", ErrUnavailable, err)
	}
	c.provider = provider
	return provider, nil
}

// oauth2Config 返回 Web 客户端的授权码流程配置。
func (c *Client) oauth2Config(provider *oidc.Provider, redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.config.WebClientID,
		ClientSecret: c.config.WebClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURI,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
}
