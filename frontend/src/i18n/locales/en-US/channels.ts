/** 美式英语·渠道文案。 */
const channels = {
  types: {
    website: "Website",
    telegram: "Telegram",
    wechatOfficialAccount: "WeChat Official Account",
  },
  loading: "Loading…",
  retry: "Try again",
  locales: {
    zhCN: "Simplified Chinese",
    enUS: "English",
  },
  filters: {
    search: "Search channel names",
    category: "Channel category",
    allCategories: "All categories",
    status: "Channel status",
    clear: "Clear filters",
  },
  statuses: {
    enabled: "Enabled",
    disabled: "Disabled",
  },
  tabs: {
    basic: "Basic information",
    reception: "Reception settings",
    chatInterface: "Chat interface",
    usage: "Integration",
  },
  list: {
    title: "Message channels",
    create: "Add channel",
    edit: "Edit",
    more: "More",
    activate: "Activate",
    deactivate: "Deactivate",
    statusUpdateError: "Could not change the channel status. Try again.",
    loadError: "Could not load message channels.",
    emptyTitle: "No message channels yet",
    emptyFiltered: "No channels match these filters",
    columns: {
      name: "Name",
      category: "Channel category",
      language: "Default service language",
      actions: "Actions",
    },
  },
  deactivation: {
    title: "Deactivate “{{name}}”?",
    description: "You can activate it again later.",
    confirm: "Deactivate",
  },
  activation: {
    title: "Activate “{{name}}”?",
    description: "The channel status will change to enabled.",
    confirm: "Activate",
  },
  statusConfirmation: {
    cancel: "Cancel",
  },
  create: {
    title: "Add channel",
  },
  edit: {
    title: "{{type}} channel settings",
    fallbackTitle: "Message channel settings",
    namedTitle: "{{type}} · {{name}}",
  },
  form: {
    type: "Channel category",
    name: "Channel name",
    description: "Description",
    defaultLocale: "Default service language",
    save: "Save",
    saving: "Saving…",
    saved: "Basic information saved.",
    cancel: "Cancel",
    back: "Back",
    loadError: "Could not load the message channel.",
    networkError: "Could not connect to the server. Try again later.",
  },
  routing: {
    newConversation: "New conversations go to",
    fallback: "When unavailable",
    select: "Select",
    person: "Company member",
    agent: "AI employee",
    loadError: "Could not load teams and members. Try again.",
    saved: "Reception settings saved.",
    targetLabels: {
      newConversation: {
        team: "Receiving team",
        member: "Receiving member",
      },
      fallback: {
        team: "Transfer team",
        member: "Transfer member",
      },
    },
    newConversationTypes: {
      public_queue: "Public queue",
      team: "Specific team",
      member: "Specific member",
    },
    fallbackTypes: {
      public_queue: "Return to public queue",
      team: "Transfer to a specific team",
      member: "Transfer to a specific member",
    },
  },
  validation: {
    nameRequired: "Enter a channel name.",
    nameTooLong: "The channel name cannot exceed 100 characters.",
    descriptionTooLong:
      "The channel description cannot exceed 2000 characters.",
    teamRequired: "Select a team.",
    memberRequired: "Select a member.",
    fallbackDifferent: "The fallback cannot use the same team or member.",
  },
  usage: {
    embed: "Website embed",
    link: "Chat link",
    snippet: "Install code",
    snippetHelp:
      "Add this code to your website. A chat button will appear in the bottom-right corner.",
    allowedHosts: "Allowed websites",
    allowedHostsHelp:
      "Enter one domain per line. Leave this blank or enter * to allow every website.",
    chatUrl: "Chat link",
    chatUrlHelp: "Visitors can open this link to enter the chat.",
    qrCode: "QR code",
    qrCodeHelp: "Visitors can scan this code to open the chat link.",
    qrCodeAlt: "Chat link QR code",
    qrCodeLoading: "Generating…",
    qrCodeFailed: "Could not generate the QR code. Try again.",
    copy: "Copy",
    copied: "Copied",
    copyFailed: "Could not copy. Copy the text manually.",
    originError: "Could not generate the public entry. Try again later.",
    saved: "Allowed websites saved.",
    validation: {
      allowedHostsTooMany: "You cannot allow more than 50 websites.",
      allowedHostInvalid:
        "Enter one valid domain, HTTP(S) URL, or * per line.",
    },
    instructions: {
      open: "View instructions",
      embedTitle: "How to add this to a website",
      embedDescription:
        "Add the install code to your website so visitors can start a conversation.",
      addCode: "Add the install code",
      addCodeHelp:
        "Copy the code below into every page that needs the chat entry, preferably before </body>.",
      customButton: "Use your website's button",
      customButtonHelp:
        "A button on the page can also open this chat window by using the attribute below.",
      contactButton: "Contact us",
      verifyEmbed: "Publish and verify",
      verifyEmbedHelp:
        "Publish the website, open the page, and confirm that the bottom-right entry and page button open the chat.",
      linkTitle: "How to use the chat link",
      linkDescription:
        "Share the chat link directly, use it on a website button, or present it as a QR code.",
      shareLink: "Share the link",
      shareLinkHelp:
        "Send the link below to visitors or use it as the destination of a website button.",
      useQrCode: "Use the QR code",
      useQrCodeHelp:
        "Display the QR code on this page so visitors can scan it and enter the chat.",
    },
  },
  chatInterface: {
    saved: "Chat interface saved.",
    form: {
      title: "Chat title",
      subtitle: "Subtitle (optional)",
      greetingMessage: "Greeting (optional)",
      themeColor: "Theme color",
      colorPicker: "Choose theme color",
    },
    validation: {
      titleRequired: "Enter a chat title.",
      titleTooLong: "The chat title cannot exceed 100 characters.",
      subtitleTooLong: "The subtitle cannot exceed 120 characters.",
      greetingTooLong: "The greeting cannot exceed 500 characters.",
      themeColorInvalid: "Enter a valid six-digit hexadecimal color.",
    },
    preview: {
      title: "Live preview",
      frameTitle: "Visitor Messenger preview",
      loading: "Loading visitor Messenger…",
      loadFailed: "Could not load the visitor Messenger preview.",
      retry: "Try again",
    },
  },
}

export default channels
