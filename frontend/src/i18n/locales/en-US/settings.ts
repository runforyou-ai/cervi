/** 美式英语·设置文案。 */
const settings = {
  personalTitle: "Personal settings",
  personalNavigationLabel: "Personal settings menu",
  systemTitle: "System settings",
  systemNavigationLabel: "System settings menu",
  navigation: {
    profile: "Profile",
    security: "Login & security",
    preferences: "Preferences",
    general: "General",
    roles: "Roles and permissions",
  },
  profile: {
    title: "Profile",
    formLabel: "Profile form",
    displayName: "Name",
    email: "Email",
    avatar: "Profile image",
    avatarChoose: "Choose image",
    avatarUploadError: "Could not upload the profile image. Try again.",
    saveSuccess: "Profile saved.",
    saveError: "Could not save the profile. Try again.",
    validation: {
      displayNameRequired: "Enter your name.",
      displayNameInvalid: "Names can only contain letters, numbers, spaces, and · - _ . characters.",
      emailRequired: "Enter your email.",
      emailInvalid: "Enter a valid email address.",
    },
  },
  password: {
    formLabel: "Change password form",
    currentPassword: "Current password",
    newPassword: "New password",
    confirmPassword: "Confirm password",
    save: "Change password",
    saving: "Changing…",
    saveSuccess: "Password changed.",
    saveError: "Could not change the password. Try again.",
    validation: {
      currentPasswordRequired: "Enter your current password.",
      newPasswordRequired: "Enter a new password.",
      newPasswordTooShort:
        "The new password must contain at least 8 characters.",
      newPasswordTooLong: "The new password cannot exceed 72 UTF-8 bytes.",
      confirmPasswordRequired: "Enter the new password again.",
      passwordMismatch: "The new passwords do not match.",
    },
  },
  security: {
    title: "Login & security",
  },
  preferences: {
    title: "Preferences",
    formLabel: "Preferences form",
    language: "Language",
    timeZone: "Time zone",
    languages: {
      zhCN: "简体中文",
      enUS: "English",
    },
    workspace: {
      title: "Workspace",
      tabs: "Use multiple tabs",
      tabsDescription:
        "Keep multiple feature pages open, or switch within the current page when disabled.",
    },
    notifications: {
      title: "Notifications",
      newMessages: "New message notifications",
      newMessagesDescription:
        "Cervi notifies you about new messages while you are working. Notifications pause while you are taking a break or off work.",
      sound: "Play the system default notification sound",
      soundDescription:
        "Only affects this device and plays the system default sound for new messages.",
      permission: {
        label: "Notifications on this device",
        authorized: "Authorized",
        authorizedDescription:
          "This device can show new message notifications.",
        unauthorizedDescription:
          "Authorize notifications before Cervi can show them on this device.",
        allow: "Allow notifications",
        allowing: "Requesting…",
        allowSuccess: "Notification permission enabled.",
        allowDenied:
          "Notifications were not enabled. Allow them in your browser or system settings.",
        allowError: "Could not request notification permission. Try again.",
        settingsOpenError:
          "Could not open notification settings. Open them manually in system settings.",
      },
    },
    saveSuccess: "Preferences saved.",
    saveError: "Could not save preferences. Try again.",
    validation: {
      timeZoneRequired: "Select a time zone.",
    },
  },
  appearance: {
    theme: "Theme",
    options: {
      system: "System",
      light: "Light",
      dark: "Dark",
    },
  },
  general: {
    title: "General",
    saveError: "Could not save general settings. Try again.",
    saveSuccess: "General settings saved.",
    form: {
      name: "Company name",
      allowArbitraryUrl: "Allow opening any URL",
    },
    confirm: {
      title: "Allow opening any URL?",
      description:
        "Members will be able to visit sites that the company has not configured or verified. This may expose them to phishing, malicious content, or sensitive-data leaks. Confirm that your company has appropriate usage policies.",
      enable: "Enable",
    },
    validation: {
      nameRequired: "Enter the company name.",
      nameTooLong: "The company name cannot exceed 32 characters.",
    },
  },
  roles: {
    title: "Roles and permissions",
    kindsDescriptions: {
      admin: "Manages the company and can use every feature",
      customerService:
        "Serves customers and handles their questions and conversations",
      member: "Collaborates internally and connects with other team members",
    },
    list: {
      create: "New role",
      loadError: "Could not load roles.",
      limitReached: "A company cannot have more than 20 roles.",
      empty: "No roles yet",
      permissionEmpty: "No permissions",
      permissionSeparator: ", ",
      permissionSummary: "{{items}} and {{count}} permissions total",
      columns: {
        name: "Role",
        description: "Description",
        memberCount: "Members",
        permissions: "Permissions",
      },
    },
    form: {
      createTitle: "New role",
      detailTitle: "Role details",
      name: "Role name",
      description: "Description",
      loadError: "Could not load the role.",
      createSuccess: "Role created.",
      updateSuccess: "Role saved.",
      saveError: "Could not save the role. Try again.",
    },
    permissions: {
      memberTitle: "Member permissions",
      adminDescription: "Administrators always have every permission.",
      label: "{{level}} {{resource}}",
      toggle: "{{resource}}: {{level}}",
      columns: {
        function: "Feature",
        view: "View",
        manage: "Manage",
      },
      resources: {
        externalContacts: "External contacts",
        teamMembers: "Team members",
        channels: "Channels",
        roles: "Roles and permissions",
        organization: "General settings",
      },
    },
    members: {
      sectionTitle: "Role members",
      count: "{{count}} members",
      configure: "Configure members",
      title: "Configure role members: {{role}}",
      newRole: "New role",
      searchAria: "Search members",
      selected: "Added ({{count}})",
      loading: "Loading members…",
      loadError: "Could not load members.",
      emptyAvailable: "No members are available",
      emptySelected: "No members have been added",
      noSearchResults: "No matching members",
      assignedTo: "Already in “{{role}}”",
      aiEmployee: "AI employee",
      add: "Add",
      confirm: "Confirm",
    },
    validation: {
      nameRequired: "Enter a role name.",
      nameTooLong: "The role name cannot exceed 10 characters.",
      descriptionTooLong:
        "The role description cannot exceed 200 characters.",
    },
    delete: {
      title: "Delete “{{name}}”?",
      description: "This action cannot be undone.",
      success: "Role deleted.",
      error: "Could not delete the role. Try again.",
    },
  },
}

export default settings
