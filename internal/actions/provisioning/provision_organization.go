//go:build server

// Package provisioning 实现官方托管的企业开通与域名前缀查询。
package provisioning

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commontimezone "github.com/runforyou-ai/cervi/internal/common/timezone"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

var (
	ErrDomainPrefixInvalid = errors.New("domain prefix is invalid")
	ErrDomainTaken         = errors.New("domain is taken")
	ErrIdempotencyConflict = errors.New("provisioning id was used with different input")
	ErrNotFound            = errors.New("provisioning not found")
)

// provisioningIDPattern 限定开通标识为 1 到 128 位字母、数字、下划线或连字符，可原样用作路径参数。
var provisioningIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

const (
	subjectMaxLength  = 255
	planCodeMaxLength = 64
)

// ValidationCode 标识开通请求字段的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationProvisioningIDInvalid    ValidationCode = "PROVISIONING_ID_INVALID"
	ValidationOrganizationNameRequired ValidationCode = "PROVISIONING_ORGANIZATION_NAME_REQUIRED"
	ValidationOrganizationNameTooLong  ValidationCode = "PROVISIONING_ORGANIZATION_NAME_TOO_LONG"
	ValidationSubjectInvalid           ValidationCode = "PROVISIONING_SUBJECT_INVALID"
	ValidationDisplayNameRequired      ValidationCode = "PROVISIONING_DISPLAY_NAME_REQUIRED"
	ValidationDisplayNameInvalid       ValidationCode = "PROVISIONING_DISPLAY_NAME_INVALID"
	ValidationEmailInvalid             ValidationCode = "PROVISIONING_EMAIL_INVALID"
	ValidationLocaleInvalid            ValidationCode = "PROVISIONING_LOCALE_INVALID"
	ValidationTimeZoneInvalid          ValidationCode = "PROVISIONING_TIME_ZONE_INVALID"
	ValidationRevisionInvalid          ValidationCode = "PROVISIONING_ENTITLEMENT_REVISION_INVALID"
	ValidationPlanCodeInvalid          ValidationCode = "PROVISIONING_PLAN_CODE_INVALID"
)

// ValidationError 表示开通请求字段校验失败。
type ValidationError = common.FieldError

// InitialUserInput 定义开通企业的初始成员。
type InitialUserInput struct {
	Subject     string
	DisplayName string
	Email       string
	Locale      domain.Locale
	TimeZone    string
}

// EntitlementInput 定义开通企业的初始权益，ServiceEndsAt 为空表示长期有效。
type EntitlementInput struct {
	Revision      int64
	PlanCode      string
	ServiceEndsAt *time.Time
}

// Input 定义运营开通企业请求。
type Input struct {
	ProvisioningID   string
	OrganizationName string
	DomainPrefix     string
	InitialUser      InitialUserInput
	Entitlement      EntitlementInput
}

// Result 描述开通操作创建的企业及其当前状态。
type Result struct {
	ProvisioningID      string
	OrganizationID      string
	AccessHost          string
	InitialUserID       string
	LifecycleStatus     domain.OrganizationLifecycleStatus
	EntitlementRevision int64
}

// ProvisionOrganizationAction 按开通标识幂等地创建托管企业。
type ProvisionOrganizationAction struct {
	db           *bun.DB
	issuer       string
	domainSuffix string
}

// NewProvisionOrganizationAction 创建企业开通操作，issuer 与域名后缀取自可信部署配置。
func NewProvisionOrganizationAction(db *bun.DB, issuer, domainSuffix string) *ProvisionOrganizationAction {
	return &ProvisionOrganizationAction{db: db, issuer: issuer, domainSuffix: domainSuffix}
}

// Execute 校验开通请求并在单个事务中创建企业、初始成员与外部身份绑定、初始权益和幂等记录。
func (a *ProvisionOrganizationAction) Execute(ctx context.Context, input Input) (Result, error) {
	input = normalizeInput(input)
	if !domain.DomainPrefixValid(input.DomainPrefix) {
		return Result{}, ErrDomainPrefixInvalid
	}
	if fields := validateInput(input); len(fields) > 0 {
		return Result{}, &ValidationError{Fields: fields}
	}
	digest, err := requestDigest(input)
	if err != nil {
		return Result{}, err
	}

	result, err := a.provision(ctx, input, digest)
	// 并发提交同一开通标识时后到的事务因唯一约束失败，按已提交的开通记录判定结果。
	if errors.Is(err, organizationaction.ErrAccessHostTaken) || pgerr.UniqueViolationOn(err, "operator_provisionings_pkey") {
		existing, storedDigest, lookupErr := loadResult(ctx, a.db, input.ProvisioningID)
		switch {
		case lookupErr == nil:
			return matchDigest(existing, storedDigest, digest)
		case !errors.Is(lookupErr, ErrNotFound):
			return Result{}, lookupErr
		case errors.Is(err, organizationaction.ErrAccessHostTaken):
			return Result{}, ErrDomainTaken
		}
	}
	if err != nil {
		return Result{}, fmt.Errorf("provision organization: %w", err)
	}
	return result, nil
}

// provision 在事务中返回已有开通结果或创建企业。
func (a *ProvisionOrganizationAction) provision(ctx context.Context, input Input, digest string) (Result, error) {
	var result Result
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing, storedDigest, err := loadResult(ctx, tx, input.ProvisioningID)
		if err == nil {
			result, err = matchDigest(existing, storedDigest, digest)
			return err
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}

		identity, err := organizationaction.Create(ctx, tx, organizationaction.CreateInput{
			AccessHost:       domain.ManagedAccessHost(input.DomainPrefix, a.domainSuffix),
			Name:             input.OrganizationName,
			AdminDisplayName: input.InitialUser.DisplayName,
			AdminEmail:       input.InitialUser.Email,
			Locale:           input.InitialUser.Locale,
			TimeZone:         input.InitialUser.TimeZone,
		})
		if err != nil {
			return err
		}
		organizationID, userID := identity.Organization.ID, identity.User.ID
		binding := &servermodels.ExternalIdentity{
			OrganizationID: organizationID,
			UserID:         userID,
			Issuer:         a.issuer,
			Subject:        input.InitialUser.Subject,
		}
		if _, err := tx.NewInsert().Model(binding).
			Column("organization_id", "user_id", "issuer", "subject").
			Exec(ctx); err != nil {
			return err
		}
		entitlement := &servermodels.OrganizationEntitlement{
			OrganizationID: organizationID,
			Revision:       input.Entitlement.Revision,
			PlanCode:       input.Entitlement.PlanCode,
			ServiceEndsAt:  input.Entitlement.ServiceEndsAt,
		}
		if _, err := tx.NewInsert().Model(entitlement).
			Column("organization_id", "revision", "plan_code", "service_ends_at").
			Exec(ctx); err != nil {
			return err
		}
		record := &servermodels.OperatorProvisioning{
			ProvisioningID: input.ProvisioningID,
			RequestDigest:  digest,
			OrganizationID: organizationID,
			InitialUserID:  userID,
		}
		if _, err := tx.NewInsert().Model(record).
			Column("provisioning_id", "request_digest", "organization_id", "initial_user_id").
			Exec(ctx); err != nil {
			return err
		}
		result = Result{
			ProvisioningID:      input.ProvisioningID,
			OrganizationID:      organizationID,
			AccessHost:          identity.Organization.AccessHost,
			InitialUserID:       userID,
			LifecycleStatus:     domain.OrganizationLifecycleStatus(identity.Organization.LifecycleStatus),
			EntitlementRevision: input.Entitlement.Revision,
		}
		return nil
	})
	return result, err
}

// matchDigest 在请求摘要一致时返回已有开通结果，否则返回幂等冲突。
func matchDigest(existing Result, storedDigest, digest string) (Result, error) {
	if storedDigest != digest {
		return Result{}, ErrIdempotencyConflict
	}
	return existing, nil
}

// loadResult 读取开通记录对应企业的当前状态和请求摘要。
func loadResult(ctx context.Context, db bun.IDB, provisioningID string) (Result, string, error) {
	var row struct {
		Result
		RequestDigest string
	}
	err := db.NewSelect().
		TableExpr("operator_provisionings AS op").
		Join("JOIN organizations AS o ON o.id = op.organization_id").
		Join("JOIN organization_entitlements AS oe ON oe.organization_id = op.organization_id").
		ColumnExpr("op.provisioning_id, op.organization_id::text, op.initial_user_id::text, op.request_digest").
		ColumnExpr("o.access_host, o.lifecycle_status, oe.revision AS entitlement_revision").
		Where("op.provisioning_id = ?", provisioningID).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, "", ErrNotFound
	}
	if err != nil {
		return Result{}, "", err
	}
	return row.Result, row.RequestDigest, nil
}

// normalizeInput 规范化开通请求中的文本、邮箱、前缀和时间；官方身份 subject 按 OIDC 逐字符比较规则保留原值。
func normalizeInput(input Input) Input {
	input.ProvisioningID = strings.TrimSpace(input.ProvisioningID)
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	input.DomainPrefix = domain.NormalizeDomainPrefix(input.DomainPrefix)
	input.InitialUser.DisplayName = strings.TrimSpace(input.InitialUser.DisplayName)
	input.InitialUser.Email = commonemail.Normalize(input.InitialUser.Email)
	input.Entitlement.PlanCode = strings.TrimSpace(input.Entitlement.PlanCode)
	if input.Entitlement.ServiceEndsAt != nil {
		endsAt := input.Entitlement.ServiceEndsAt.UTC().Truncate(time.Microsecond)
		input.Entitlement.ServiceEndsAt = &endsAt
	}
	return input
}

// validateInput 校验前缀以外的开通字段。
func validateInput(input Input) map[string]ValidationCode {
	fields := make(map[string]ValidationCode)
	if !provisioningIDPattern.MatchString(input.ProvisioningID) {
		fields["provisioningId"] = ValidationProvisioningIDInvalid
	}
	if input.OrganizationName == "" {
		fields["organizationName"] = ValidationOrganizationNameRequired
	} else if utf8.RuneCountInString(input.OrganizationName) > domain.OrganizationNameMaxLength {
		fields["organizationName"] = ValidationOrganizationNameTooLong
	}
	if input.InitialUser.Subject == "" || len(input.InitialUser.Subject) > subjectMaxLength {
		fields["initialUser.subject"] = ValidationSubjectInvalid
	}
	if input.InitialUser.DisplayName == "" {
		fields["initialUser.displayName"] = ValidationDisplayNameRequired
	} else if !domain.IdentityDisplayNameValid(input.InitialUser.DisplayName) {
		fields["initialUser.displayName"] = ValidationDisplayNameInvalid
	}
	if !commonemail.Valid(input.InitialUser.Email) {
		fields["initialUser.email"] = ValidationEmailInvalid
	}
	if input.InitialUser.Locale != domain.LocaleChineseSimplified && input.InitialUser.Locale != domain.LocaleEnglishUnitedStates {
		fields["initialUser.locale"] = ValidationLocaleInvalid
	}
	if !commontimezone.Valid(input.InitialUser.TimeZone) {
		fields["initialUser.timeZone"] = ValidationTimeZoneInvalid
	}
	if input.Entitlement.Revision < 1 {
		fields["entitlement.revision"] = ValidationRevisionInvalid
	}
	if input.Entitlement.PlanCode == "" || len(input.Entitlement.PlanCode) > planCodeMaxLength {
		fields["entitlement.planCode"] = ValidationPlanCodeInvalid
	}
	return fields
}

// requestDigest 计算规范化开通请求的 SHA-256 摘要。
func requestDigest(input Input) (string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode provisioning request: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
