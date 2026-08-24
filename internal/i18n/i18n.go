// Package i18n 提供后端用户可见文案的本地化能力。
package i18n

import (
	"embed"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Key 标识一条后端本地化文案。
type Key string

const (
	AppProductName                        Key = "app.product_name"
	AppTrayOpen                           Key = "app.tray_open"
	AppTrayQuit                           Key = "app.tray_quit"
	ErrorInternal                         Key = "error.internal"
	ErrorMethodNotAllowed                 Key = "error.method_not_allowed"
	ErrorInstallationStatusReadFailed     Key = "error.installation_status_read_failed"
	ErrorAlreadyInitialized               Key = "error.already_initialized"
	ErrorInstallationRequired             Key = "error.installation_required"
	ErrorAuthenticationStatusFailed       Key = "error.authentication_status_failed"
	ErrorAuthenticationRequired           Key = "error.authentication_required"
	ErrorValidationFailed                 Key = "error.validation_failed"
	ErrorInstallationFailed               Key = "error.installation_failed"
	ErrorInvalidCredentials               Key = "error.invalid_credentials"
	ErrorLoginFailed                      Key = "error.login_failed"
	ErrorLogoutFailed                     Key = "error.logout_failed"
	ErrorChannelNotFound                  Key = "error.channel_not_found"
	ErrorChannelListFailed                Key = "error.channel_list_failed"
	ErrorChannelReadFailed                Key = "error.channel_read_failed"
	ErrorChannelCreateFailed              Key = "error.channel_create_failed"
	ErrorChannelUpdateFailed              Key = "error.channel_update_failed"
	ErrorChannelChatInterfaceUpdateFailed Key = "error.channel_chat_interface_update_failed"
	ErrorChannelAccessUpdateFailed        Key = "error.channel_access_update_failed"
	ErrorContactNotFound                  Key = "error.contact_not_found"
	ErrorContactListFailed                Key = "error.contact_list_failed"
	ErrorContactReadFailed                Key = "error.contact_read_failed"
	ErrorContactCreateFailed              Key = "error.contact_create_failed"
	ErrorContactUpdateFailed              Key = "error.contact_update_failed"
	ErrorContactDeleteFailed              Key = "error.contact_delete_failed"
	ErrorContactRestoreFailed             Key = "error.contact_restore_failed"
	ErrorUserNotFound                     Key = "error.user_not_found"
	ErrorUserListFailed                   Key = "error.user_list_failed"
	ErrorUserReadFailed                   Key = "error.user_read_failed"
	ErrorUserCreateFailed                 Key = "error.user_create_failed"
	ErrorAgentCreateFailed                Key = "error.agent_create_failed"
	ErrorAgentNotFound                    Key = "error.agent_not_found"
	ErrorAgentReadFailed                  Key = "error.agent_read_failed"
	ErrorAgentUpdateFailed                Key = "error.agent_update_failed"
	ErrorAgentStatusUpdateFailed          Key = "error.agent_status_update_failed"
	ErrorUserUpdateFailed                 Key = "error.user_update_failed"
	ErrorUserStatusUpdateFailed           Key = "error.user_status_update_failed"
	ErrorUserLastActiveAdministrator      Key = "error.user_last_active_administrator"
	ErrorTeamNotFound                     Key = "error.team_not_found"
	ErrorTeamListFailed                   Key = "error.team_list_failed"
	ErrorTeamCreateFailed                 Key = "error.team_create_failed"
	ErrorTeamUpdateFailed                 Key = "error.team_update_failed"
	ErrorTeamDeleteFailed                 Key = "error.team_delete_failed"
	ErrorTeamMemberListFailed             Key = "error.team_member_list_failed"
	ErrorTeamMemberAddFailed              Key = "error.team_member_add_failed"
	ErrorTeamMemberNotFound               Key = "error.team_member_not_found"
	ErrorTeamMemberRemoveFailed           Key = "error.team_member_remove_failed"
	ErrorRoleNotFound                     Key = "error.role_not_found"
	ErrorRoleListFailed                   Key = "error.role_list_failed"
	ErrorRoleReadFailed                   Key = "error.role_read_failed"
	ErrorRoleCreateFailed                 Key = "error.role_create_failed"
	ErrorRoleUpdateFailed                 Key = "error.role_update_failed"
	ErrorRoleDeleteFailed                 Key = "error.role_delete_failed"
	ErrorRoleAdminImmutable               Key = "error.role_admin_immutable"
	ErrorRoleBuiltInDeleteForbidden       Key = "error.role_builtin_delete_forbidden"
	ErrorRoleLimitReached                 Key = "error.role_limit_reached"
	ErrorRoleInUse                        Key = "error.role_in_use"
	ErrorAIProviderNotFound               Key = "error.ai_provider_not_found"
	ErrorAIProviderListFailed             Key = "error.ai_provider_list_failed"
	ErrorAIProviderReadFailed             Key = "error.ai_provider_read_failed"
	ErrorAIProviderCreateFailed           Key = "error.ai_provider_create_failed"
	ErrorAIProviderUpdateFailed           Key = "error.ai_provider_update_failed"
	ErrorAIProviderDeleteFailed           Key = "error.ai_provider_delete_failed"
	ErrorProfileUpdateFailed              Key = "error.profile_update_failed"
	ErrorFileUploadCreateFailed           Key = "error.file_upload_create_failed"
	ErrorFileUploadCompleteFailed         Key = "error.file_upload_complete_failed"
	ErrorFileNotFound                     Key = "error.file_not_found"
	ErrorPasswordUpdateFailed             Key = "error.password_update_failed"
	ErrorPreferencesUpdateFailed          Key = "error.preferences_update_failed"
	ErrorWorkStatusUpdateFailed           Key = "error.work_status_update_failed"
	ErrorChannelSummaryListFailed         Key = "error.channel_summary_list_failed"
	ErrorOrganizationUpdateFailed         Key = "error.organization_update_failed"
	ErrorS3SettingReadFailed              Key = "error.s3_setting_read_failed"
	ErrorS3SettingSaveFailed              Key = "error.s3_setting_save_failed"
	ErrorS3ConnectionTestFailed           Key = "error.s3_connection_test_failed"
	ErrorServerURLInvalid                 Key = "error.server_url_invalid"
	ErrorServerUnavailable                Key = "error.server_unavailable"
	ErrorServerConnectionSaveFailed       Key = "error.server_connection_save_failed"
	ErrorServerConnectionRequired         Key = "error.server_connection_required"
	ErrorServerInitializationRequired     Key = "error.server_initialization_required"
	ErrorRemoteRequestCreateFailed        Key = "error.remote_request_create_failed"
	ErrorServerConnectionFailed           Key = "error.server_connection_failed"

	FieldOrganizationNameRequired    Key = "field.organization_name_required"
	FieldOrganizationNameTooLong     Key = "field.organization_name_too_long"
	FieldDisplayNameRequired         Key = "field.display_name_required"
	FieldAgentNameRequired           Key = "field.agent_name_required"
	FieldEmailInvalid                Key = "field.email_invalid"
	FieldEmailDuplicate              Key = "field.email_duplicate"
	FieldPasswordTooShort            Key = "field.password_too_short"
	FieldPasswordTooLong             Key = "field.password_too_long"
	FieldCurrentPasswordIncorrect    Key = "field.current_password_incorrect"
	FieldLocaleInvalid               Key = "field.locale_invalid"
	FieldTimeZoneInvalid             Key = "field.time_zone_invalid"
	FieldWorkStatusInvalid           Key = "field.work_status_invalid"
	FieldAgentWorkStatusUnavailable  Key = "field.agent_work_status_unavailable"
	FieldMemberRoleInvalid           Key = "field.member_role_invalid"
	FieldUserTeamInvalid             Key = "field.user_team_invalid"
	FieldMemberTeamInvalid           Key = "field.member_team_invalid"
	FieldUserStatusInvalid           Key = "field.user_status_invalid"
	FieldTeamNameRequired            Key = "field.team_name_required"
	FieldTeamNameTooLong             Key = "field.team_name_too_long"
	FieldTeamNameDuplicate           Key = "field.team_name_duplicate"
	FieldTeamDescriptionTooLong      Key = "field.team_description_too_long"
	FieldTeamQueryInvalid            Key = "field.team_query_invalid"
	FieldFileNameRequired            Key = "field.file_name_required"
	FieldFileContentTypeInvalid      Key = "field.file_content_type_invalid"
	FieldFileByteSizeInvalid         Key = "field.file_byte_size_invalid"
	FieldFilePurposeInvalid          Key = "field.file_purpose_invalid"
	FieldRoleNameRequired            Key = "field.role_name_required"
	FieldRoleNameTooLong             Key = "field.role_name_too_long"
	FieldRoleNameDuplicate           Key = "field.role_name_duplicate"
	FieldRoleDescriptionTooLong      Key = "field.role_description_too_long"
	FieldRolePermissionsInvalid      Key = "field.role_permissions_invalid"
	FieldAIProviderBrandInvalid      Key = "field.ai_provider_brand_invalid"
	FieldAIProviderNameRequired      Key = "field.ai_provider_name_required"
	FieldAIProviderNameTooLong       Key = "field.ai_provider_name_too_long"
	FieldAIProviderNameDuplicate     Key = "field.ai_provider_name_duplicate"
	FieldAIProviderAPIKeyRequired    Key = "field.ai_provider_api_key_required"
	FieldAIProviderAPIKeyTooLong     Key = "field.ai_provider_api_key_too_long"
	FieldAIProviderAPIURLRequired    Key = "field.ai_provider_api_url_required"
	FieldAIProviderAPIURLInvalid     Key = "field.ai_provider_api_url_invalid"
	FieldAIProviderModelsInvalid     Key = "field.ai_provider_models_invalid"
	FieldChannelTypeInvalid          Key = "field.channel_type_invalid"
	FieldChannelNameRequired         Key = "field.channel_name_required"
	FieldChannelNameTooLong          Key = "field.channel_name_too_long"
	FieldChannelDescriptionTooLong   Key = "field.channel_description_too_long"
	FieldChannelDefaultLocaleInvalid Key = "field.channel_default_locale_invalid"
	FieldChannelRoutingTargetInvalid Key = "field.channel_routing_target_invalid"
	FieldChannelChatTitleRequired    Key = "field.channel_chat_title_required"
	FieldChannelChatTitleTooLong     Key = "field.channel_chat_title_too_long"
	FieldChannelChatSubtitleTooLong  Key = "field.channel_chat_subtitle_too_long"
	FieldChannelGreetingTooLong      Key = "field.channel_greeting_too_long"
	FieldChannelThemeColorInvalid    Key = "field.channel_theme_color_invalid"
	FieldChannelAllowedHostsTooMany  Key = "field.channel_allowed_hosts_too_many"
	FieldChannelAllowedHostInvalid   Key = "field.channel_allowed_host_invalid"
	FieldContactIdentityRequired     Key = "field.contact_identity_required"
	FieldContactChannelRequired      Key = "field.contact_channel_required"
	FieldContactChannelInvalid       Key = "field.contact_channel_invalid"
	FieldContactChannelImmutable     Key = "field.contact_channel_immutable"
	FieldContactNameTooLong          Key = "field.contact_name_too_long"
	FieldContactStageInvalid         Key = "field.contact_stage_invalid"
	FieldContactNotesTooLong         Key = "field.contact_notes_too_long"
	FieldContactMethodsTooMany       Key = "field.contact_methods_too_many"
	FieldContactMethodInvalid        Key = "field.contact_method_invalid"
	FieldContactMethodDuplicate      Key = "field.contact_method_duplicate"
	FieldContactPrimaryDuplicate     Key = "field.contact_primary_duplicate"
	FieldContactQueryInvalid         Key = "field.contact_query_invalid"
	FieldS3EndpointRequired          Key = "field.s3_endpoint_required"
	FieldS3EndpointInvalid           Key = "field.s3_endpoint_invalid"
	FieldS3ProviderInvalid           Key = "field.s3_provider_invalid"
	FieldS3RegionRequired            Key = "field.s3_region_required"
	FieldS3BucketRequired            Key = "field.s3_bucket_required"
	FieldS3AccessKeyIDRequired       Key = "field.s3_access_key_id_required"
	FieldS3SecretAccessKeyRequired   Key = "field.s3_secret_access_key_required"
	FieldQueryPositiveInteger        Key = "field.query_positive_integer"
	FieldServerURLComplete           Key = "field.server_url_complete"
	FieldServerURLBaseOnly           Key = "field.server_url_base_only"
	FieldServerURLHTTPSRequired      Key = "field.server_url_https_required"
	FieldServerURLNotCervi           Key = "field.server_url_not_cervi"
)

//go:embed locales/*.json
var localeFiles embed.FS

var bundle = loadBundle()

// Localize 根据语言偏好返回本地化文案和最终匹配的语言。
func Localize(acceptLanguage string, key Key) (string, string) {
	localizer := goi18n.NewLocalizer(bundle, acceptLanguage)
	message, tag, err := localizer.LocalizeWithTag(&goi18n.LocalizeConfig{MessageID: string(key)})
	if err != nil {
		panic(err)
	}
	return message, tag.String()
}

// LocalizeMap 将一组命名文案键翻译为对应文案。
func LocalizeMap(acceptLanguage string, keys map[string]Key) map[string]string {
	if len(keys) == 0 {
		return nil
	}
	localizer := goi18n.NewLocalizer(bundle, acceptLanguage)
	messages := make(map[string]string, len(keys))
	for name, key := range keys {
		messages[name] = localizer.MustLocalize(&goi18n.LocalizeConfig{MessageID: string(key)})
	}
	return messages
}

// loadBundle 加载嵌入到二进制中的翻译词条。
func loadBundle() *goi18n.Bundle {
	bundle := goi18n.NewBundle(language.AmericanEnglish)
	if _, err := bundle.LoadMessageFileFS(localeFiles, "locales/en-US.json"); err != nil {
		panic(err)
	}
	if _, err := bundle.LoadMessageFileFS(localeFiles, "locales/zh-CN.json"); err != nil {
		panic(err)
	}
	return bundle
}
