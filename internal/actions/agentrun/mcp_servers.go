//go:build server

package agentrun

import (
	"context"
	"fmt"
	"slices"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// runMCPServers 表示本次运行挂载的 MCP 服务，以及是否有按客户查询的服务因客户未验证而未挂载。
type runMCPServers struct {
	Servers               []agentruntime.MCPServer
	CustomerLoginRequired bool
}

// loadRunMCPServers 读取本次运行的配置版本绑定且仍存在的同企业 MCP 服务。
// 按客户查询的服务只在客服场景挂载：客户已验证时附加客户请求头，未验证时不挂载。
// 客服场景只挂载标记为查询的工具，按客户查询的服务排在前面，与其他服务同名的工具保留带客户请求头的一个。
func loadRunMCPServers(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (runMCPServers, error) {
	services := make([]servermodels.MCPServer, 0)
	err := db.NewSelect().Model(&services).
		Join("JOIN agent_revisions AS ar ON ar.id = ? AND ar.organization_id = ms.organization_id", run.AgentRevisionID).
		Where("ms.organization_id = ?", run.OrganizationID).
		Where("ar.configuration->'mcpServerIds' @> jsonb_build_array(ms.id::text)").
		OrderExpr("ms.customer_scoped DESC, ms.name").Scan(ctx)
	if err != nil {
		return runMCPServers{}, err
	}
	loaded := runMCPServers{Servers: make([]agentruntime.MCPServer, 0, len(services))}
	customerScene := domain.AgentExecutionScopeKind(run.ScopeKind) == domain.AgentExecutionScopeServiceSession
	var customer *runCustomer
	for _, service := range services {
		server := agentruntime.MCPServer{Name: service.Name, Config: mcpintegration.Config{
			URL: service.URL, ServerType: service.ServerType, AuthorizationToken: service.AuthorizationToken,
		}}
		if !customerScene {
			if !service.CustomerScoped {
				loaded.Servers = append(loaded.Servers, server)
			}
			continue
		}
		// 按名称顺序收集查询工具，没有查询工具的服务不挂载。
		server.Tools = make([]string, 0, len(service.ToolPurposes))
		for name, purpose := range service.ToolPurposes {
			if purpose == domain.MCPToolPurposeQuery {
				server.Tools = append(server.Tools, name)
			}
		}
		if len(server.Tools) == 0 {
			continue
		}
		slices.Sort(server.Tools)
		if service.CustomerScoped {
			if customer == nil {
				if customer, err = loadRunCustomer(ctx, db, run); err != nil {
					return runMCPServers{}, err
				}
			}
			if customer.UserID == "" {
				loaded.CustomerLoginRequired = true
				continue
			}
			server.Config.Headers = map[string]string{mcpintegration.CustomerIDHeader: customer.UserID}
			if customer.Email != "" {
				server.Config.Headers[mcpintegration.CustomerEmailHeader] = customer.Email
			}
		}
		loaded.Servers = append(loaded.Servers, server)
	}
	return loaded, nil
}

// runCustomer 表示客服周期的已验证客户，未验证时企业用户编号为空。
type runCustomer struct {
	UserID string
	Email  string
}

// loadRunCustomer 读取客服周期渠道身份对应的已验证客户与其主要邮箱，判定与客户上下文消息一致。
func loadRunCustomer(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (*runCustomer, error) {
	row := struct {
		ExternalID     string  `bun:"external_id"`
		ExternalUserID *string `bun:"external_user_id"`
		Email          *string `bun:"email"`
	}{}
	if err := db.NewSelect().
		TableExpr("service_sessions AS ss").
		ColumnExpr("cci.external_id, c.external_user_id").
		ColumnExpr("(SELECT cm.value FROM contact_methods AS cm WHERE cm.organization_id = c.organization_id AND cm.contact_id = c.id AND cm.type = ? AND cm.is_primary) AS email", domain.ContactMethodTypeEmail).
		Join("JOIN contact_channel_identities AS cci ON cci.id = ss.contact_channel_identity_id AND cci.organization_id = ss.organization_id").
		Join("JOIN contacts AS c ON c.id = cci.contact_id AND c.organization_id = cci.organization_id").
		Where("ss.organization_id = ?", run.OrganizationID).
		Where("ss.id = ?", run.ScopeID).
		Scan(ctx, &row); err != nil {
		return nil, fmt.Errorf("load run customer: %w", err)
	}
	customer := &runCustomer{}
	if row.ExternalUserID != nil && conversationaction.IsWebsiteCustomerExternalID(row.ExternalID) {
		customer.UserID = *row.ExternalUserID
		if row.Email != nil {
			customer.Email = *row.Email
		}
	}
	return customer, nil
}
