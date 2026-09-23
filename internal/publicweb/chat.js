// 网站渠道访客 Messenger 交互。
(function () {
  var messenger = document.getElementById("cv-messenger");
  var messages = document.getElementById("cv-messages");
  var followingMessages = true;
  var previousMessagesHeight = messages.scrollHeight;
  var messageResizeObserver = new ResizeObserver(function () {
    if (followingMessages) messages.scrollTop = messages.scrollHeight;
    previousMessagesHeight = messages.scrollHeight;
  });
  // 当前会话消息增删后重新划分消息组。
  var messageGroupObserver = new MutationObserver(refreshMessageGroups);
  messageGroupObserver.observe(messages, { childList: true });
  messages.addEventListener("scroll", function () {
    if (messages.scrollHeight === previousMessagesHeight) {
      followingMessages = messages.scrollHeight - messages.scrollTop - messages.clientHeight <= 48;
    }
    previousMessagesHeight = messages.scrollHeight;
  });
  window.addEventListener("pagehide", function (event) {
    if (event.persisted) return;
    haltVisitorRealtime("stopped");
    messageResizeObserver.disconnect();
    messageGroupObserver.disconnect();
    CerviMarkdown.unmount(messages);
    conversationItems.forEach(function (conversation) { CerviMarkdown.unmount(conversation.fragment); });
  });
  var composer = document.getElementById("cv-composer");
  var input = document.getElementById("cv-input");

  var MAX_ATTACHMENT_COUNT = 10;
  var COMPOSER_MAX_HEIGHT = 200;
  var REALTIME_PROTOCOL_VERSION = 1;
  var REALTIME_IDLE_TIMEOUT = 60000;
  var REALTIME_BACKOFF_BASE = 1000;
  var REALTIME_BACKOFF_MAX = 30000;
  var TYPING_EXPIRY = 6000;
  var TYPING_REFRESH = 3000;
  var TYPING_IDLE = 5000;
  var TYPING_STOP_GRACE = 4000;
  var previewMode = messenger.getAttribute("data-preview") === "true";
  var channelID = messenger.getAttribute("data-channel-id");
  var visitorToken = "";
  var initialized = previewMode;
  var initializationPending = false;
  var messageRequestPending = false;
  var conversationItems = [];
  var typingConversation = null;
  var typingReportedAt = 0;
  var typingIdleTimer = 0;
  var conversationByID = Object.create(null);
  var activeRoute = "home";
  var conversationReturnRoute = "home";
  var activeConversation = createConversation();
  var recentConversation = null;
  var realtimeState = "idle";
  var realtimeAttempt = 0;
  var realtimeFailures = 0;
  var realtimeClose = null;
  var realtimeTimer = null;
  var directoryRefreshing = false;
  var directoryRefreshQueued = false;
  var refreshRetryTimer = null;
  var refreshFailures = 0;
  var recordingStartedAt = 0;
  var recordingTimer = null;
  var lightbox = null;
  var parentOrigin = document.referrer ? new URL(document.referrer).origin : "";
  var messengerVisible =
    !document.documentElement.classList.contains("cv-embed");
  var expanded = false;
  var nativeContextMenuSource = "";
  var defaultTitle = document.title;
  var displayTitle = defaultTitle;
  var demoReply = messenger.getAttribute("data-demo-reply");
  var playVoiceLabel = messenger.getAttribute("data-play-voice");
  var pauseVoiceLabel = messenger.getAttribute("data-pause-voice");
  var expandWindowLabel = messenger.getAttribute("data-expand-window");
  var collapseWindowLabel = messenger.getAttribute("data-collapse-window");
  var defaultGreeting = messenger.getAttribute("data-default-greeting");
  var defaultSubtitle = messenger.getAttribute("data-default-subtitle");
  var loadingLabel = messenger.getAttribute("data-loading");
  var replyMenu = document.getElementById("cv-message-menu");
  var replyMenuSource = null;
  var referenceNavigationSeq = 0;
  var referenceLabels = {
    reply: messenger.getAttribute("data-reference-reply"),
    replying: messenger.getAttribute("data-reference-replying"),
    unavailable: messenger.getAttribute("data-reference-unavailable"),
    deleted: messenger.getAttribute("data-reference-deleted"),
    visitor: messenger.getAttribute("data-reference-visitor"),
    agent: messenger.getAttribute("data-reference-agent"),
  };
  var requestFailedLabel = messenger.getAttribute("data-request-failed");
  var attachmentLabels = {
    uploading: messenger.getAttribute("data-attachment-uploading"),
    failed: messenger.getAttribute("data-attachment-failed"),
    cancel: messenger.getAttribute("data-attachment-cancel"),
    receiving: messenger.getAttribute("data-attachment-receiving"),
    unavailable: messenger.getAttribute("data-attachment-unavailable"),
    retry: messenger.getAttribute("data-retry"),
  };
  var sessionLabels = {
    open: messenger.getAttribute("data-session-open"),
    closed: messenger.getAttribute("data-session-closed"),
  };
  var dayLabels = {
    today: messenger.getAttribute("data-day-today"),
    yesterday: messenger.getAttribute("data-day-yesterday"),
  };
  var eventLabels = {
    sessionEnded: messenger.getAttribute("data-session-ended"),
    memberJoined: messenger.getAttribute("data-member-joined"),
  };
  var ratingLabels = {
    question: messenger.getAttribute("data-rating-question"),
    resolved: messenger.getAttribute("data-rating-resolved"),
    unresolved: messenger.getAttribute("data-rating-unresolved"),
    comment: messenger.getAttribute("data-rating-comment"),
    submit: messenger.getAttribute("data-rating-submit"),
    thanks: messenger.getAttribute("data-rating-thanks"),
  };
  var RATING_COMMENT_MAX_LENGTH = 1000;
  var RATING_COMMENT_MAX_HEIGHT = 120;
  var emojiPanel = document.getElementById("cv-emoji");
  var moreMenu = document.getElementById("cv-more-menu");
  var moreToggle = document.getElementById("cv-more-toggle");
  var sendButton = document.getElementById("cv-send");
  var fileInput = document.getElementById("cv-file-input");
  var composerMain = document.getElementById("cv-composer-main");
  var recording = document.getElementById("cv-recording");
  var recordTime = document.getElementById("cv-record-time");
  var intro = document.getElementById("cv-conversation-intro");
  var unreadDot = document.getElementById("cv-unread-dot");
  var emojis = window.CERVI_COMPOSER_EMOJIS;
  if (parentOrigin === "null") {
    parentOrigin = "";
  }

  function $(id) {
    return document.getElementById(id);
  }

  function syncMoreAvailability() {
    moreToggle.hidden = !$("cv-expand") || $("cv-expand").hidden;
  }

  function navigate(route) {
    if (route !== "conversation" && !recording.hidden) {
      resetRecording(false);
    }
    document.querySelectorAll("[data-screen]").forEach(function (screen) {
      screen.hidden = screen.getAttribute("data-screen") !== route;
    });
    referenceNavigationSeq += 1;
    activeRoute = route;
    var topLevel = route === "home" || route === "messages" || route === "help";
    $("cv-navigation").hidden = !topLevel;
    document.querySelectorAll("[data-route-target]").forEach(function (button) {
      if (button.closest(".cv-navigation")) {
        if (button.getAttribute("data-route-target") === route) {
          button.setAttribute("aria-current", "page");
        } else {
          button.removeAttribute("aria-current");
        }
      }
    });
    closeOverlays();
    if (route === "conversation") {
      clearUnread();
      window.setTimeout(function () {
        input.focus();
        scrollToBottom();
      }, 0);
    } else {
      var heading = document.querySelector(
        '[data-screen="' + route + '"] [data-route-heading]',
      );
      if (heading) {
        window.setTimeout(function () {
          heading.focus();
        }, 0);
      }
    }
  }

  function createConversation(summary) {
    summary = summary || null;
    return {
      id: summary ? summary.id : null,
      title: summary ? summary.title : "",
      fragment: document.createDocumentFragment(),
      started: summary !== null,
      draft: "",
      summary: summary ? CerviMarkdown.preview(summary.preview, summary.previewSenderIdentityType) : "",
      time: summary ? formatTime(new Date(summary.lastMessageAt)) : "",
      lastMessageAt: summary ? summary.lastMessageAt : "",
      serviceSession: summary ? summary.serviceSession : null,
      unread: false,
      after: "",
      before: "",
      replyTo: null,
      pendingReplyToID: "",
      refreshing: false,
      refreshSeq: 0,
      refreshPending: false,
      sessionRatings: Object.create(null),
      lastMessageSeq: summary ? summary.lastMessageSeq : "0",
      replyState: "none",
      typingNode: null,
      typingTimer: 0,
      historyLoaded: false,
      historyLoading: false,
      messageIDs: Object.create(null),
      pendingMessageID: "",
      pendingBody: "",
      pendingAttachments: [],
      attachmentSending: false,
      creating: null,
    };
  }

  // 返回指定会话当前承载消息的容器。
  function conversationMessageContainer(conversation) {
    return conversation === activeConversation
      ? messages
      : conversation.fragment;
  }

  // 向指定会话追加节点，并只滚动当前会话。
  function appendConversationNode(conversation, node) {
    conversationMessageContainer(conversation).appendChild(node);
    if (conversation === activeConversation && followingMessages) {
      scrollToBottom();
    }
  }

  function stashActiveConversation() {
    stopTypingReport();
    activeConversation.draft = input.value;
    Array.from(messages.children).forEach(function (node) {
      if (node !== intro) {
        activeConversation.fragment.appendChild(node);
      }
    });
  }

  function showConversation(conversation) {
    if (activeConversation === conversation) {
      if (
        !previewMode &&
        conversation.id &&
        !conversation.historyLoaded &&
        !conversation.historyLoading
      ) {
        loadConversationHistory(conversation);
      } else if (conversation.historyLoaded) {
        refreshActiveConversationMessages();
      }
      return;
    }
    referenceNavigationSeq += 1;
    stashActiveConversation();
    activeConversation = conversation;
    messages.appendChild(activeConversation.fragment);
    intro.hidden = previewMode
      ? activeConversation.started
      : activeConversation.id !== null;
    $("cv-conversation-error").hidden = true;
    input.value = activeConversation.draft;
    renderComposerReference();
    fileInput.value = "";
    if (!recording.hidden) {
      resetRecording(false);
    }
    closeOverlays();
    if (lightbox) {
      lightbox.hidden = true;
    }
    autosize();
    updateSendState();
    // 切换会话后重新跟随正文的异步布局和后续消息。
    followingMessages = true;
    messages.scrollTop = activeConversation.started ? messages.scrollHeight : 0;
    if (
      !previewMode &&
      activeConversation.id &&
      !activeConversation.historyLoaded
    ) {
      loadConversationHistory(activeConversation);
    } else if (activeConversation.historyLoaded) {
      refreshActiveConversationMessages();
    }
  }

  function beginNewConversation() {
    if (!previewMode && !initialized) {
      return;
    }
    var returnRoute = activeRoute;
    showConversation(createConversation());
    conversationReturnRoute = returnRoute;
    navigate("conversation");
  }

  function resumeRecentConversation() {
    if (!recentConversation) {
      return;
    }
    var returnRoute = activeRoute;
    showConversation(recentConversation);
    conversationReturnRoute = returnRoute;
    navigate("conversation");
  }

  // 打开指定会话。
  function resumeConversation(conversation) {
    var returnRoute = activeRoute;
    showConversation(conversation);
    conversationReturnRoute = returnRoute;
    navigate("conversation");
  }

  function postToParent(message) {
    if (window.parent === window) {
      return;
    }
    window.parent.postMessage(message, parentOrigin || "*");
  }

  function closeMessenger() {
    if (!recording.hidden) {
      resetRecording(false);
    }
    postToParent({ type: "cervi:close" });
  }

  function closeOverlays() {
    replyMenu.hidden = true;
    emojiPanel.hidden = true;
    $("cv-emoji-toggle").setAttribute("aria-expanded", "false");
    moreMenu.hidden = true;
    moreToggle.setAttribute("aria-expanded", "false");
  }

  // 记录浏览器菜单是否由真实鼠标右键触发。
  function rememberNativeContextMenuGesture(event) {
    nativeContextMenuSource =
      event.button === 2 && !event.ctrlKey ? "mouse" : "";
  }

  // 记录浏览器菜单的键盘等价操作。
  function rememberKeyboardContextMenuGesture(event) {
    nativeContextMenuSource =
      event.key === "ContextMenu" || (event.key === "F10" && event.shiftKey)
        ? "keyboard"
        : "";
  }

  // 只允许鼠标右键、键盘或触摸打开原生上下文菜单。
  function allowOnlyNativeSecondaryButtonMenu(event) {
    replyMenu.hidden = true;
    var allow =
      nativeContextMenuSource !== "" ||
      (event.pointerType && event.pointerType !== "mouse");
    nativeContextMenuSource = "";
    if (!allow) {
      event.preventDefault();
    }
  }

  function showHelpTopic(button) {
    $("cv-help-detail-title").textContent =
      button.querySelector("strong").textContent;
    navigate("help-detail");
  }

  function filterHelp() {
    var query = $("cv-help-input").value.trim().toLocaleLowerCase();
    var visible = 0;
    document
      .querySelectorAll("#cv-collection-list [data-search-text]")
      .forEach(function (button) {
        var searchText = button
          .getAttribute("data-search-text")
          .toLocaleLowerCase();
        button.hidden = query !== "" && searchText.indexOf(query) === -1;
        if (!button.hidden) {
          visible += 1;
        }
      });
    $("cv-help-empty").hidden = visible !== 0;
  }

  function autosize() {
    input.style.height = "auto";
    var contentHeight = Math.min(
      input.scrollHeight,
      COMPOSER_MAX_HEIGHT,
    );
    input.style.height = contentHeight + "px";
    var renderedHeight = input.getBoundingClientRect().height;
    input.style.overflowY =
      input.scrollHeight > renderedHeight ? "auto" : "hidden";
  }

  function updateSendState() {
    var blocked =
      !previewMode &&
      (!initialized ||
        messageRequestPending ||
        activeConversation.historyLoading ||
        activeConversation.creating !== null);
    var empty = input.value.trim() === "";
    sendButton.disabled = empty || blocked;
    // 输入内容后录音入口换成发送按钮，左侧附件入口保持可用；输入框宽度变化后重新计算高度。
    if (sendButton.hidden !== empty) {
      sendButton.hidden = empty;
      $("cv-voice").hidden = !empty;
      autosize();
    }
    if (!previewMode) {
      $("cv-attach").disabled = !initialized || activeConversation.historyLoading;
    }
  }

  function formatTime(date) {
    return date.toLocaleTimeString(document.documentElement.lang, {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  // 按服务端时间和会话编号稳定比较会话新旧。
  function compareConversationRecency(left, right) {
    var leftTime = normalizedTimestamp(left.lastMessageAt);
    var rightTime = normalizedTimestamp(right.lastMessageAt);
    if (leftTime !== rightTime) {
      return leftTime < rightTime ? 1 : -1;
    }
    return (right.id || "").localeCompare(left.id || "");
  }

  // normalizedTimestamp 统一 RFC3339 小数秒精度。
  function normalizedTimestamp(value) {
    var match = value.match(/^(.*?)(?:\.(\d+))?Z$/);
    return match[1] + "." + (match[2] || "").padEnd(9, "0").slice(0, 9) + "Z";
  }

  // 无损比较同一会话中的消息序号。
  function compareMessagePosition(leftSeq, rightSeq) {
    var left = BigInt(leftSeq);
    var right = BigInt(rightSeq);
    return left < right ? -1 : left > right ? 1 : 0;
  }

  function formatDuration(seconds) {
    var minutes = Math.floor(seconds / 60);
    return minutes + ":" + String(seconds % 60).padStart(2, "0");
  }

  function formatSize(bytes) {
    if (bytes < 1024) {
      return bytes + " B";
    }
    if (bytes < 1024 * 1024) {
      return (bytes / 1024).toFixed(1) + " KB";
    }
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  }

  function scrollToBottom() {
    followingMessages = true;
    $("cv-latest-message").hidden = true;
    messages.scrollTop = messages.scrollHeight;
  }

  var MESSAGE_GROUP_INTERVAL = 5 * 60 * 1000;
  var AGENT_AVATAR_ICON =
    '<svg viewBox="0 0 24 24"><path d="M12 8V4H8" /><rect width="16" height="12" x="4" y="8" rx="2" /><path d="M2 14h2M20 14h2M15 13v2M9 13v2" /></svg>';
  var PERSON_AVATAR_ICON =
    '<svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="5" /><path d="M20 21a8 8 0 0 0-16 0" /></svg>';

  // 创建带发送人资料和发送时间的消息节点。
  function messageContainer(author, date, sender) {
    var message = document.createElement("article");
    message.className = "cv-message cv-message-" + author;
    sender = sender || {};
    message.setAttribute("data-sender-key", author + ":" + (sender.key || ""));
    message.setAttribute("data-sender-name", sender.name || "");
    message.setAttribute("data-sender-avatar", sender.avatarURL || "");
    message.setAttribute("data-sender-fallback", sender.fallback || (author === "visitor" ? "person" : "agent"));
    message.setAttribute("data-originated-at", String(date.getTime()));
    return message;
  }

  // 创建消息时间标签。
  function messageTime(date) {
    var time = document.createElement("time");
    time.className = "cv-message-time";
    time.dateTime = date.toISOString();
    time.textContent = formatTime(date);
    return time;
  }

  // 创建气泡外的消息时间行，用于无气泡的纯附件消息。
  function messageMeta(date) {
    var meta = document.createElement("div");
    meta.className = "cv-message-meta";
    meta.appendChild(messageTime(date));
    return meta;
  }

  // 创建气泡正文区，时间浮动在正文末行右端。
  function bubbleContent(body, date) {
    var content = document.createElement("div");
    content.className = "cv-message-body";
    body.classList.add("cv-message-text");
    content.appendChild(body);
    content.appendChild(messageTime(date));
    return content;
  }

  // 判断两条相邻消息是否属于同一发送人的连续消息组。
  function sameMessageGroup(previous, next) {
    if (!previous || !next || previous.getAttribute("data-sender-key") !== next.getAttribute("data-sender-key")) {
      return false;
    }
    var previousTime = Number(previous.getAttribute("data-originated-at"));
    var nextTime = Number(next.getAttribute("data-originated-at"));
    var interval = nextTime - previousTime;
    return (
      new Date(previousTime).toDateString() === new Date(nextTime).toDateString() &&
      interval >= 0 &&
      interval <= MESSAGE_GROUP_INTERVAL
    );
  }

  // 返回日期分割线文案：今天、昨天，今年的只显示月日，其余显示完整日期。
  function dayLabel(date) {
    var now = new Date();
    var yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
    if (date.toDateString() === now.toDateString()) {
      return dayLabels.today;
    }
    if (date.toDateString() === yesterday.toDateString()) {
      return dayLabels.yesterday;
    }
    var options = date.getFullYear() === now.getFullYear()
      ? { month: "long", day: "numeric" }
      : { year: "numeric", month: "long", day: "numeric" };
    return date.toLocaleDateString(document.documentElement.lang, options);
  }

  // 让每天第一条消息前恰有一条日期分割线；只增删不符合的节点，重复执行不产生新变更，避免消息组观察器反复触发。
  function refreshDayDividers(items) {
    var starts = new Set();
    items.forEach(function (message, index) {
      var time = Number(message.getAttribute("data-originated-at"));
      var previous = items[index - 1];
      if (!Number.isFinite(time)) {
        return;
      }
      if (!previous || new Date(Number(previous.getAttribute("data-originated-at"))).toDateString() !== new Date(time).toDateString()) {
        starts.add(message);
      }
    });
    Array.from(messages.querySelectorAll(":scope > .cv-day-divider")).forEach(function (divider) {
      if (!starts.has(divider.nextElementSibling)) {
        divider.remove();
      }
    });
    starts.forEach(function (message) {
      var date = new Date(Number(message.getAttribute("data-originated-at")));
      var divider = message.previousElementSibling;
      if (!divider || !divider.classList.contains("cv-day-divider")) {
        divider = document.createElement("div");
        divider.className = "cv-day-divider";
        divider.appendChild(document.createElement("time"));
        messages.insertBefore(divider, message);
      }
      var label = divider.firstElementChild;
      var text = dayLabel(date);
      if (label.textContent !== text) {
        label.dateTime = date.toISOString();
        label.textContent = text;
      }
    });
  }

  // 按发送人和时间间隔划分当前会话消息组，只在组内最后一条显示头像，并按天插入日期分割线。
  function refreshMessageGroups() {
    var items = Array.from(messages.children).filter(function (node) {
      return node.classList.contains("cv-message") || node.classList.contains("cv-event");
    });
    refreshDayDividers(items);
    items.forEach(function (message, index) {
      var endsGroup = !sameMessageGroup(message, items[index + 1]);
      message.toggleAttribute("data-group-start", !sameMessageGroup(items[index - 1], message));
      message.toggleAttribute("data-group-end", endsGroup);
      var row = message.querySelector(".cv-message-row");
      var avatar = row && row.querySelector(":scope > .cv-message-avatar");
      if (endsGroup && row && !avatar) {
        row.appendChild(messageAvatar(message));
      } else if (!endsGroup && avatar) {
        avatar.remove();
      }
    });
  }

  // 按发送人资料生成头像，图片不可用时显示默认图案或姓名首字。
  function messageAvatar(message) {
    var avatar = document.createElement("span");
    avatar.className = "cv-message-avatar";
    avatar.setAttribute("aria-hidden", "true");
    var name = message.getAttribute("data-sender-name").trim();
    var imageURL = message.getAttribute("data-sender-avatar");
    var fallback = message.getAttribute("data-sender-fallback");
    if (name) {
      avatar.title = name;
    }
    var showFallback = function () {
      var initial = Array.from(name)[0];
      avatar.textContent = "";
      if (fallback === "person" && initial) {
        var letter = document.createElement("span");
        letter.textContent = initial.toLocaleUpperCase();
        avatar.appendChild(letter);
        return;
      }
      avatar.innerHTML = fallback === "agent" ? AGENT_AVATAR_ICON : PERSON_AVATAR_ICON;
    };
    if (!imageURL) {
      showFallback();
      return avatar;
    }
    var image = document.createElement("img");
    image.alt = "";
    image.draggable = false;
    image.src = imageURL;
    image.addEventListener("error", showFallback);
    avatar.appendChild(image);
    return avatar;
  }

  function startConversation() {
    if (activeConversation.started) {
      return;
    }
    activeConversation.started = true;
    intro.hidden = true;
    var greeting = document
      .querySelector("[data-channel-greeting]")
      .textContent.trim();
    if (greeting) {
      appendAssistantMessage(activeConversation, greeting, true);
    }
  }

  function appendVisitorMessage(text, files) {
    if (!text && files.length === 0) {
      return;
    }
    startConversation();
    var now = new Date();
    var message = messageContainer("visitor", now, { key: "self" });
    var row = document.createElement("div");
    row.className = "cv-message-row";
    if (text) {
      var bubble = document.createElement("div");
      bubble.className = "cv-message-bubble";
      var paragraph = document.createElement("div");
      paragraph.textContent = text;
      bubble.appendChild(bubbleContent(paragraph, now));
      row.appendChild(bubble);
    }
    if (files.length > 0) {
      row.classList.add("cv-message-row-with-assets");
      row.appendChild(assetList(files));
    }
    message.appendChild(row);
    if (!text) {
      message.appendChild(messageMeta(now));
    }
    messages.appendChild(message);
    updateConversationSummary(activeConversation, text || files[0].name, now);
    scrollToBottom();
  }

  // 向指定会话追加客服消息。
  function appendAssistantMessage(conversation, text, greeting) {
    if (!text) {
      return;
    }
    var now = new Date();
    var message = messageContainer("assistant", now, { key: "channel" });
    if (greeting) {
      message.setAttribute("data-greeting", "true");
    }
    var row = document.createElement("div");
    row.className = "cv-message-row";
    var bubble = document.createElement("div");
    bubble.className = "cv-message-bubble";
    var body = document.createElement("div");
    CerviMarkdown.render(body, text, greeting ? null : "agent");
    bubble.appendChild(bubbleContent(body, now));
    messageResizeObserver.observe(bubble);
    row.appendChild(bubble);
    message.appendChild(row);
    appendConversationNode(conversation, message);
    if (!greeting) {
      updateConversationSummary(conversation, CerviMarkdown.preview(text, "agent"), now);
    }
  }

  // 向指定会话追加正在输入提示。
  function appendTyping(conversation) {
    var message = messageContainer("assistant", new Date(), { key: "channel" });
    var row = document.createElement("div");
    row.className = "cv-message-row";
    var typing = document.createElement("div");
    typing.className = "cv-typing";
    typing.innerHTML = "<i></i><i></i><i></i>";
    row.appendChild(typing);
    message.appendChild(row);
    conversation.typingNode = message;
    appendConversationNode(conversation, message);
  }

  // 按收到的输入状态显示或清除指定会话的正在输入提示。
  // 访客事件不携带发送者，客服与 AI 员工共用同一个提示：停止时保留一个长于刷新间隔的宽限期，
  // 仍在准备回复的一方会在宽限期内把提示续上，都停止后到期清除。
  function applyConversationTyping(conversation, active) {
    if (!active && !conversation.typingNode) {
      return;
    }
    window.clearTimeout(conversation.typingTimer);
    if (active) {
      if (conversation.typingNode) {
        // 提示始终留在消息列表末尾。
        conversationMessageContainer(conversation).appendChild(conversation.typingNode);
        if (conversation === activeConversation && followingMessages) {
          scrollToBottom();
        }
      } else {
        appendTyping(conversation);
      }
    }
    conversation.typingTimer = window.setTimeout(function () {
      removeConversationTyping(conversation);
    }, active ? TYPING_EXPIRY : TYPING_STOP_GRACE);
  }

  // 清除指定会话的正在输入提示。
  function removeConversationTyping(conversation) {
    window.clearTimeout(conversation.typingTimer);
    conversation.typingTimer = 0;
    if (!conversation.typingNode) {
      return;
    }
    conversation.typingNode.remove();
    conversation.typingNode = null;
  }

  // 为当前会话安排互不干扰的演示回复。
  function scheduleDemoReply() {
    if (!demoReply || activeConversation.replyState !== "none") {
      return;
    }
    var conversation = activeConversation;
    conversation.replyState = "pending";
    window.setTimeout(function () {
      appendTyping(conversation);
    }, 320);
    window.setTimeout(function () {
      removeConversationTyping(conversation);
      conversation.replyState = "sent";
      appendAssistantMessage(conversation, demoReply, false);
    }, 980);
  }

  function renderRecentConversation() {
    var hasRecentConversation = recentConversation !== null;
    $("cv-messages-empty").hidden = hasRecentConversation;
    $("cv-conversation-list").hidden = !hasRecentConversation;
    $("cv-home-recent").hidden = !hasRecentConversation;
    $("cv-conversation-list").innerHTML = "";
    if (!hasRecentConversation) {
      unreadDot.hidden = true;
      postToParent({ type: "cervi:unread", unread: false });
      return;
    }
    conversationItems.forEach(function (conversation) {
      $("cv-conversation-list").appendChild(
        conversationListButton(conversation),
      );
    });
    $("cv-home-recent").querySelector("strong").textContent = previewMode
      ? displayTitle
      : recentConversation.title;
    $("cv-home-recent-preview").textContent = recentConversation.summary;
    $("cv-home-recent-time").textContent = recentConversation.time;
    unreadDot.hidden = previewMode ? !recentConversation.unread : true;
    $("cv-home-recent-unread-dot").hidden = !recentConversation.unread;
    postToParent({
      type: "cervi:unread",
      unread: previewMode && recentConversation.unread,
    });
  }

  // 创建一条可点击的会话列表项。
  function conversationListButton(conversation) {
    var button = document.createElement("button");
    button.type = "button";
    button.setAttribute("data-conversation-id", conversation.id || "preview");
    var avatar = document.createElement("span");
    avatar.className = "cv-presence-avatar";
    avatar.setAttribute("aria-hidden", "true");
    var initials = document.createElement("span");
    initials.textContent = document.querySelector(
      ".cv-presence-avatar span",
    ).textContent;
    avatar.appendChild(initials);
    var summary = document.createElement("span");
    summary.className = "cv-conversation-summary";
    var titleRow = document.createElement("span");
    titleRow.className = "cv-conversation-row";
    var title = document.createElement("strong");
    title.textContent = previewMode ? displayTitle : conversation.title;
    var time = document.createElement("time");
    time.textContent = conversation.time;
    titleRow.appendChild(title);
    titleRow.appendChild(time);
    var previewRow = document.createElement("span");
    previewRow.className = "cv-conversation-row";
    var preview = document.createElement("span");
    preview.textContent = conversation.summary;
    previewRow.appendChild(preview);
    if (!previewMode && conversation.serviceSession) {
      var status = document.createElement("small");
      status.textContent =
        sessionLabels[conversation.serviceSession.status] || "";
      previewRow.appendChild(status);
    }
    summary.appendChild(titleRow);
    summary.appendChild(previewRow);
    button.appendChild(avatar);
    button.appendChild(summary);
    button.addEventListener("click", function () {
      resumeConversation(conversation);
    });
    return button;
  }

  // 返回消息在会话列表中的摘要，纯附件消息取文件名。
  function messagePreview(message) {
    if (!message.body && message.attachment) {
      return message.attachment.name;
    }
    return CerviMarkdown.preview(message.body, message.senderIdentityType);
  }

  // 返回消息页中最后一条对话消息，客服处理周期事件不计入会话摘要。
  function lastDialogueMessage(items) {
    for (var index = items.length - 1; index >= 0; index -= 1) {
      if (items[index].author !== "system") {
        return items[index];
      }
    }
    return null;
  }

  // 更新指定会话的摘要、时间和未读状态。
  function updateConversationSummary(conversation, preview, date, messageSeq) {
    var originatedAt =
      typeof date === "string" ? date : date.toISOString();
    if (previewMode) messageSeq = String(BigInt(conversation.lastMessageSeq) + 1n);
    if (compareMessagePosition(messageSeq, conversation.lastMessageSeq) < 0) return;
    conversation.summary = preview;
    conversation.time = formatTime(new Date(originatedAt));
    conversation.lastMessageAt = originatedAt;
    conversation.lastMessageSeq = messageSeq;
    conversation.unread =
      conversation !== activeConversation ||
      activeRoute !== "conversation" ||
      !messengerVisible;
    if (conversationItems.indexOf(conversation) === -1) {
      conversationItems.push(conversation);
    }
    conversationItems.sort(compareConversationRecency);
    recentConversation = conversationItems[0];
    renderRecentConversation();
  }

  function clearUnread() {
    if (!recentConversation || activeConversation !== recentConversation) {
      return;
    }
    recentConversation.unread = false;
    renderRecentConversation();
  }

  function sendMessage() {
    var text = input.value.trim();
    if (!text) {
      return;
    }
    stopTypingReport();
    if (!previewMode) {
      sendRealMessage(text);
      return;
    }
    appendVisitorMessage(text, []);
    input.value = "";
    autosize();
    updateSendState();
    scheduleDemoReply();
    input.focus();
  }

  // 上报访客在指定线程中的输入状态，失败由接收端到期清除兜底。
  function postVisitorTyping(conversation, active) {
    requestWebsiteJSON(
      "/api/public/website-channels/" +
        encodeURIComponent(channelID) +
        "/conversations/" +
        encodeURIComponent(conversation.id) +
        "/typing",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active: active }),
      },
    ).catch(function () {});
  }

  // 按输入内容变化上报开始输入，持续输入按间隔刷新，内容为空时上报停止。
  function reportTypingInput() {
    if (previewMode) {
      return;
    }
    if (typingConversation && typingConversation !== activeConversation) {
      stopTypingReport();
    }
    if (!activeConversation.id || !input.value.trim()) {
      stopTypingReport();
      return;
    }
    var now = Date.now();
    if (!typingConversation || now - typingReportedAt >= TYPING_REFRESH) {
      typingConversation = activeConversation;
      typingReportedAt = now;
      postVisitorTyping(activeConversation, true);
    }
    window.clearTimeout(typingIdleTimer);
    typingIdleTimer = window.setTimeout(stopTypingReport, TYPING_IDLE);
  }

  // 结束本次输入，之前上报过开始输入时上报停止。
  function stopTypingReport() {
    window.clearTimeout(typingIdleTimer);
    typingIdleTimer = 0;
    if (!typingConversation) {
      return;
    }
    var conversation = typingConversation;
    typingConversation = null;
    postVisitorTyping(conversation, false);
  }

  // 请求网站访客 JSON 接口。
  function requestWebsiteJSON(path, options) {
    options = options || {};
    options.headers = options.headers || {};
    options.headers.Accept = "application/json";
    options.headers["Accept-Language"] = document.documentElement.lang;
    if (visitorToken) {
      options.headers["X-Cervi-Visitor-Token"] = visitorToken;
    }
    return window.fetch(path, options).then(function (response) {
      return response.json().then(function (payload) {
        if (!response.ok) {
          var message = payload.error && payload.error.message;
          var error = new Error(message || requestFailedLabel);
          throw error;
        }
        return payload;
      });
    });
  }

  // 将服务端摘要合并到页面会话状态。
  function upsertRealConversation(
    summary,
    preferredConversation,
  ) {
    var conversation = conversationByID[summary.id];
    if (
      conversation &&
      preferredConversation &&
      conversation !== preferredConversation &&
      conversation !== activeConversation
    ) {
      // 该线程已由目录刷新建立占位对象时，由发送中的会话接管，保留其消息节点和草稿。
      conversationItems.splice(conversationItems.indexOf(conversation), 1);
      conversation = null;
    }
    if (!conversation) {
      conversation = preferredConversation || createConversation(summary);
      conversation.id = summary.id;
      conversation.started = true;
      conversationByID[summary.id] = conversation;
      conversationItems.push(conversation);
    }
    conversation.title = summary.title;
    if (compareMessagePosition(summary.lastMessageSeq, conversation.lastMessageSeq) >= 0) {
      conversation.summary = CerviMarkdown.preview(summary.preview, summary.previewSenderIdentityType);
      conversation.lastMessageAt = summary.lastMessageAt;
      conversation.lastMessageSeq = summary.lastMessageSeq;
      conversation.time = formatTime(new Date(summary.lastMessageAt));
    }
    conversation.serviceSession = summary.serviceSession;
    conversationItems.sort(compareConversationRecency);
    if (conversationItems.length > 20) {
      var removed = conversationItems.pop();
      if (removed) {
        delete conversationByID[removed.id];
      }
    }
    recentConversation =
      conversationItems.length > 0 ? conversationItems[0] : null;
    return conversation;
  }

  // 初始化真实网站 Messenger。
  function initializeRealMessenger() {
    if (previewMode || initializationPending) {
      return;
    }
    initializationPending = true;
    initialized = false;
    setNewConversationAvailability(false);
    $("cv-messages-empty").hidden = true;
    $("cv-conversation-list").hidden = true;
    showInitializationState(loadingLabel, false);
    requestWebsiteJSON(
      "/api/public/website-channels/" +
        encodeURIComponent(channelID) +
        "/messenger",
    )
      .then(function (result) {
        visitorToken = result.visitorToken;
        result.conversations.forEach(function (summary) {
          upsertRealConversation(summary, null);
        });
        initialized = true;
        hideInitializationState();
        renderRecentConversation();
        setNewConversationAvailability(true);
      })
      .catch(function (error) {
        showInitializationState(error.message || requestFailedLabel, true);
      })
      .finally(function () {
        initializationPending = false;
        updateSendState();
        syncVisitorRealtime();
      });
  }

  // 显示真实入口初始化状态。
  function showInitializationState(message, retry) {
    $("cv-initialization-error").hidden = false;
    $("cv-initialization-error-message").textContent = message;
    $("cv-initialization-retry").hidden = !retry;
    $("cv-home-initialization-status").hidden = false;
    $("cv-home-initialization-message").textContent = message;
    $("cv-home-initialization-retry").hidden = !retry;
  }

  // 隐藏真实入口初始化状态。
  function hideInitializationState() {
    $("cv-initialization-error").hidden = true;
    $("cv-home-initialization-status").hidden = true;
  }

  // 切换真实入口的新会话按钮可用状态。
  function setNewConversationAvailability(available) {
    document
      .querySelectorAll("[data-new-conversation]")
      .forEach(function (button) {
        button.disabled = !available;
      });
  }

  // 加载指定真实客户会话最近的持久消息。
  function loadConversationHistory(conversation) {
    if (conversation.historyLoading || !conversation.id) {
      return;
    }
    conversation.historyLoading = true;
    if (conversation === activeConversation) {
      $("cv-conversation-error").hidden = true;
      updateSendState();
    }
    requestWebsiteJSON(
      "/api/public/website-channels/" +
        encodeURIComponent(channelID) +
        "/conversations/" +
        encodeURIComponent(conversation.id) +
        "/messages",
    )
      .then(function (result) {
        clearConversationMessages(conversation);
        result.messages.forEach(function (message) {
          appendServerMessage(conversation, message);
        });
        syncSessionRatings(conversation, result.sessionRatings);
        conversation.before = result.before || "";
        conversation.after = result.after || "";
        conversation.historyLoaded = true;
        var lastMessage = lastDialogueMessage(result.messages);
        if (lastMessage) {
          updateConversationSummary(
            conversation,
            messagePreview(lastMessage),
            lastMessage.originatedAt,
            lastMessage.messageSeq,
          );
        }
        if (conversation === activeConversation) {
          scrollToBottom();
        }
      })
      .catch(function (error) {
        if (conversation === activeConversation) {
          $("cv-conversation-error").textContent =
            error.message || requestFailedLabel;
          $("cv-conversation-error").hidden = false;
        }
      })
      .finally(function () {
        conversation.historyLoading = false;
        if (conversation === activeConversation) {
          updateSendState();
        }
        // 加载期间到达的会话变更在加载结束后补拉。
        if (conversation.refreshPending) {
          conversation.refreshPending = false;
          refreshConversationMessages(conversation);
        }
      });
  }

  // 清空指定会话现有的真实消息节点。
  function clearConversationMessages(conversation) {
    removeConversationTyping(conversation);
    CerviMarkdown.unmount(conversationMessageContainer(conversation));
    conversationMessageContainer(conversation).querySelectorAll(".cv-message-bubble").forEach(function (bubble) {
      messageResizeObserver.unobserve(bubble);
    });
    conversation.messageIDs = Object.create(null);
    if (conversation === activeConversation) {
      Array.from(messages.children).forEach(function (node) {
        if (node !== intro) {
          node.remove();
        }
      });
      return;
    }
    conversation.fragment = document.createDocumentFragment();
  }

  // 按消息序号插入持久消息节点。
  function insertServerMessageNode(conversation, node, messageSeq) {
    var container = conversationMessageContainer(conversation);
    var nextNode = null;
    container.querySelectorAll("[data-message-id]").forEach(function (current) {
      if (
        !nextNode &&
        compareMessagePosition(
          messageSeq,
          current.getAttribute("data-message-seq"),
        ) < 0
      ) {
        nextNode = current;
      }
    });
    if (nextNode) {
      container.insertBefore(node, nextNode);
    } else {
      container.appendChild(node);
    }
  }

  // 把一条持久消息有序合入指定会话。
  function appendServerMessage(conversation, value) {
    if (conversation.messageIDs[value.id]) {
      return;
    }
    if (value.author === "system") {
      appendServerEvent(conversation, value);
      return;
    }
    var originatedAt = new Date(value.originatedAt);
    // 访客消息归入本人，客服消息按发送身份区分发送人。
    var message = messageContainer(
      value.author === "visitor" ? "visitor" : "assistant",
      originatedAt,
      value.author === "visitor"
        ? { key: "self" }
        : {
            key: value.senderIdentityId,
            name: value.senderName,
            avatarURL: value.senderAvatarUrl,
            fallback: value.senderIdentityType === "agent" ? "agent" : "person",
          },
    );
    message.setAttribute("data-message-id", value.id);
    message.tabIndex = 0;
    message.setAttribute("data-message-seq", value.messageSeq);
    var row = document.createElement("div");
    row.className = "cv-message-row";
    var bubble = document.createElement("div");
    bubble.className = "cv-message-bubble";
    if (value.replyTo) {
      var reference = document.createElement(value.replyTo.deleted ? "blockquote" : "button");
      reference.className = "cv-message-reference";
      if (value.replyTo.deleted) {
        reference.textContent = referenceLabels.deleted;
      } else {
        reference.type = "button";
        reference.addEventListener("click", function () {
          locateReferencedMessage(conversation, value.replyTo.id);
        });
        var author = document.createElement("strong");
        author.textContent = referenceLabels[value.replyTo.author];
        var excerpt = document.createElement("span");
        excerpt.className = "cv-message-reference-body";
        excerpt.textContent = CerviMarkdown.preview(value.replyTo.body, value.replyTo.senderIdentityType);
        reference.appendChild(author);
        reference.appendChild(excerpt);
      }
      bubble.appendChild(reference);
    }
    // 只有说明或引用的附件消息才渲染气泡，纯附件直接展示内容并在下方显示时间。
    var withBubble = !value.attachment || value.body || value.replyTo;
    if (withBubble) {
      var body = document.createElement("div");
      CerviMarkdown.render(body, value.body, value.senderIdentityType);
      bubble.appendChild(bubbleContent(body, originatedAt));
      messageResizeObserver.observe(bubble);
      row.appendChild(bubble);
    }
    if (value.attachment) {
      row.classList.add("cv-message-row-with-assets");
      row.appendChild(serverAssetList(conversation, value));
    }
    message.appendChild(row);
    if (!withBubble) {
      message.appendChild(messageMeta(originatedAt));
    }
    // 收到的消息悬停显示回复操作，双方消息均支持右键引用。
    if (value.author === "agent") {
      var replyButton = document.createElement("button");
      replyButton.type = "button";
      replyButton.className = "cv-message-reply";
      replyButton.textContent = referenceLabels.reply;
      replyButton.addEventListener("click", function () {
        selectReplyMessage(value);
      });
      row.appendChild(replyButton);
    }
    message.addEventListener("contextmenu", function (event) {
      if (event.defaultPrevented) {
        return;
      }
      event.preventDefault();
      closeOverlays();
      replyMenuSource = message;
      replyMenu.hidden = false;
      var bounds = message.getBoundingClientRect();
      var x = event.clientX || bounds.left;
      var y = event.clientY || bounds.top;
      replyMenu.style.left = Math.max(8, Math.min(x, window.innerWidth - replyMenu.offsetWidth - 8)) + "px";
      replyMenu.style.top = Math.max(8, Math.min(y, window.innerHeight - replyMenu.offsetHeight - 8)) + "px";
      $("cv-menu-reply").focus();
    });
    conversation.messageIDs[value.id] = value;
    insertServerMessageNode(
      conversation,
      message,
      value.messageSeq,
    );
    if (value.author === "visitor") {
      // 本人消息插入后把对方的正在输入提示重新放回末尾。
      if (conversation.typingNode) {
        conversationMessageContainer(conversation).appendChild(conversation.typingNode);
      }
      return;
    }
    removeConversationTyping(conversation);
  }

  // 把客服处理周期事件以居中提示有序合入指定会话，周期结束事件按已知评价状态挂载评价卡片。
  function appendServerEvent(conversation, value) {
    var node = document.createElement("div");
    node.className = "cv-event";
    node.setAttribute("data-message-id", value.id);
    node.setAttribute("data-message-seq", value.messageSeq);
    node.setAttribute("data-sender-key", "event:" + value.id);
    node.setAttribute("data-originated-at", String(new Date(value.originatedAt).getTime()));
    var text = document.createElement("p");
    text.className = "cv-event-text";
    text.textContent =
      value.event.type === "session_ended"
        ? eventLabels.sessionEnded
        : eventLabels.memberJoined.replace("{name}", value.event.memberName);
    node.appendChild(text);
    if (value.event.type === "session_ended") {
      node.setAttribute("data-session-ended", "");
      reconcileRatingCard(conversation, node);
    }
    conversation.messageIDs[value.id] = value;
    insertServerMessageNode(conversation, node, value.messageSeq);
    // 事件插入后把对方的正在输入提示重新放回末尾。
    if (conversation.typingNode) {
      conversationMessageContainer(conversation).appendChild(conversation.typingNode);
    }
  }

  // 用服务端返回的周期评价状态替换本地记录，并收敛全部已渲染的结束事件。
  function syncSessionRatings(conversation, ratings) {
    conversation.sessionRatings = Object.create(null);
    (ratings || []).forEach(function (rating) {
      conversation.sessionRatings[rating.endMessageId] = rating;
    });
    conversationMessageContainer(conversation)
      .querySelectorAll("[data-session-ended]")
      .forEach(function (node) {
        reconcileRatingCard(conversation, node);
      });
  }

  // 按评价状态增删或更新结束事件下的评价卡片；填写中的表单在仍可评价时保持不变。
  function reconcileRatingCard(conversation, node) {
    var rating = conversation.sessionRatings[node.getAttribute("data-message-id")];
    var card = node.querySelector(".cv-rating");
    if (!rating) {
      if (card) {
        card.remove();
      }
      return;
    }
    var state = rating.rateable ? "form" : "result";
    if (card && card.getAttribute("data-state") === state) {
      return;
    }
    var next = ratingCard(conversation, rating);
    if (card) {
      card.replaceWith(next);
    } else {
      node.appendChild(next);
    }
  }

  // 创建周期结束后的评价卡片：未评价时选择是否解决并可填写评语后提交，已评价时展示结果。
  function ratingCard(conversation, rating) {
    var card = document.createElement("div");
    card.className = "cv-rating";
    if (!rating.rateable) {
      card.setAttribute("data-state", "result");
      renderRatingResult(card, rating);
      return card;
    }
    card.setAttribute("data-state", "form");
    var form = document.createElement("form");
    form.className = "cv-rating-form";
    var question = document.createElement("p");
    question.className = "cv-rating-question";
    question.textContent = ratingLabels.question;
    var choices = document.createElement("div");
    choices.className = "cv-rating-choices";
    var details = document.createElement("div");
    details.className = "cv-rating-details";
    details.hidden = true;
    var label = document.createElement("label");
    label.className = "cv-rating-label";
    var labelText = document.createElement("span");
    labelText.textContent = ratingLabels.comment;
    var comment = document.createElement("textarea");
    comment.className = "cv-rating-comment";
    comment.rows = 1;
    comment.maxLength = RATING_COMMENT_MAX_LENGTH;
    comment.addEventListener("input", function () {
      comment.style.height = "auto";
      comment.style.height = Math.min(comment.scrollHeight, RATING_COMMENT_MAX_HEIGHT) + "px";
    });
    label.appendChild(labelText);
    label.appendChild(comment);
    var submit = document.createElement("button");
    submit.type = "submit";
    submit.className = "cv-rating-submit";
    submit.textContent = ratingLabels.submit;
    var error = document.createElement("p");
    error.className = "cv-rating-error";
    error.setAttribute("role", "alert");
    error.hidden = true;
    details.appendChild(label);
    details.appendChild(submit);
    details.appendChild(error);
    var selected = null;
    // 选择是否解决后展开评语与提交入口。
    [true, false].forEach(function (resolved) {
      var choice = document.createElement("button");
      choice.type = "button";
      choice.className = "cv-rating-choice";
      choice.textContent = resolved ? ratingLabels.resolved : ratingLabels.unresolved;
      choice.setAttribute("aria-pressed", "false");
      choice.addEventListener("click", function () {
        selected = resolved;
        choices.querySelectorAll(".cv-rating-choice").forEach(function (item) {
          item.setAttribute("aria-pressed", String(item === choice));
        });
        details.hidden = false;
      });
      choices.appendChild(choice);
    });
    form.addEventListener("submit", function (submitEvent) {
      submitEvent.preventDefault();
      if (selected === null || submit.disabled) {
        return;
      }
      submit.disabled = true;
      error.hidden = true;
      requestWebsiteJSON(
        "/api/public/website-channels/" +
          encodeURIComponent(channelID) +
          "/conversations/" +
          encodeURIComponent(conversation.id) +
          "/service-sessions/" +
          encodeURIComponent(rating.serviceSessionId) +
          "/rating",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ resolved: selected, comment: comment.value }),
        },
      )
        .then(function (result) {
          conversation.sessionRatings[rating.endMessageId] = {
            serviceSessionId: rating.serviceSessionId,
            endMessageId: rating.endMessageId,
            rateable: result.rateable,
            resolved: result.resolved,
            comment: result.comment,
          };
          if (card.parentNode) {
            reconcileRatingCard(conversation, card.parentNode);
          }
        })
        .catch(function (requestError) {
          error.textContent = requestError.message || requestFailedLabel;
          error.hidden = false;
          submit.disabled = false;
          // 提交失败时拉取当前评价状态，已在别处评价或周期已重开时卡片随之更新。
          refreshConversationMessages(conversation);
        });
    });
    form.appendChild(question);
    form.appendChild(choices);
    form.appendChild(details);
    card.appendChild(form);
    return card;
  }

  // 在评价卡片中展示已提交的是否解决与评语。
  function renderRatingResult(card, rating) {
    var thanks = document.createElement("p");
    thanks.className = "cv-rating-question";
    thanks.textContent = ratingLabels.thanks;
    var result = document.createElement("p");
    result.className = "cv-rating-result";
    result.textContent = rating.resolved ? ratingLabels.resolved : ratingLabels.unresolved;
    card.appendChild(thanks);
    card.appendChild(result);
    if (rating.comment) {
      var comment = document.createElement("p");
      comment.className = "cv-rating-result-comment";
      comment.textContent = rating.comment;
      card.appendChild(comment);
    }
  }

  // 渲染服务端附件：图片内联预览并可点开灯箱，其余显示文件名、大小和下载入口。
  function serverAssetList(conversation, value) {
    var assets = document.createElement("div");
    assets.className = "cv-message-assets";
    var attachment = value.attachment;
    if (attachment.transferStatus !== "ready") {
      var pending = document.createElement("div");
      pending.className = "cv-asset cv-file-asset";
      var pendingName = document.createElement("strong");
      pendingName.textContent = attachment.name;
      var pendingStatus = document.createElement("span");
      pendingStatus.textContent =
        attachment.transferStatus === "pending"
          ? attachmentLabels.receiving
          : attachmentLabels.unavailable;
      pending.appendChild(pendingName);
      pending.appendChild(pendingStatus);
      assets.appendChild(pending);
      return assets;
    }
    if (fileKind(attachment.contentType) === "image") {
      var imageButton = document.createElement("button");
      imageButton.type = "button";
      imageButton.className = "cv-asset";
      var image = document.createElement("img");
      image.src = attachment.previewUrl;
      image.alt = attachment.name;
      var resigned = false;
      image.addEventListener("error", function () {
        if (resigned) {
          return;
        }
        // 预览地址过期后重新签发一次。
        resigned = true;
        refreshAttachmentLinks(conversation, value)
          .then(function (links) {
            image.src = links.previewUrl;
          })
          .catch(function () {});
      });
      imageButton.appendChild(image);
      imageButton.addEventListener("click", function () {
        openLightbox(image.src, attachment.name);
      });
      assets.appendChild(imageButton);
      return assets;
    }
    var fileLink = document.createElement("a");
    fileLink.className = "cv-asset cv-file-asset";
    fileLink.href = attachment.downloadUrl;
    fileLink.download = attachment.name;
    fileLink.addEventListener("click", function (event) {
      event.preventDefault();
      // 下载前重新签发地址。
      refreshAttachmentLinks(conversation, value)
        .then(function (links) {
          downloadAttachment(links.downloadUrl, attachment.name);
        })
        .catch(function () {
          downloadAttachment(fileLink.href, attachment.name);
        });
    });
    var name = document.createElement("strong");
    name.textContent = attachment.name;
    var size = document.createElement("span");
    size.textContent = formatSize(attachment.byteSize);
    fileLink.appendChild(name);
    fileLink.appendChild(size);
    assets.appendChild(fileLink);
    return assets;
  }

  // 在挂件文档之外取得附件内容，保留当前聊天界面。
  function downloadAttachment(url, name) {
    var link = document.createElement("a");
    link.href = url;
    link.download = name;
    link.target = "_blank";
    link.rel = "noopener";
    document.body.appendChild(link);
    link.click();
    link.remove();
  }

  // 重新签发指定消息附件的预览与下载地址。
  function refreshAttachmentLinks(conversation, value) {
    return requestWebsiteJSON(
      "/api/public/website-channels/" + encodeURIComponent(channelID) +
      "/conversations/" + encodeURIComponent(conversation.id) +
      "/messages/" + encodeURIComponent(value.id) + "/attachment",
    );
  }

  // 把选中的原文保存在当前会话草稿中。
  function selectReplyMessage(value) {
    activeConversation.replyTo = value;
    closeOverlays();
    renderComposerReference();
    input.focus();
  }

  // 同步输入框的一层原文摘要。
  function renderComposerReference() {
    var value = activeConversation.replyTo;
    $("cv-composer-reference").hidden = !value;
    $("cv-composer-reference-author").textContent = value
      ? referenceLabels.replying + " " + referenceLabels[value.author]
      : "";
    $("cv-composer-reference-body").textContent = value ? CerviMarkdown.preview(value.body, value.senderIdentityType) : "";
  }

  // 补齐较早的历史后定位原文，保留连续消息和当前阅读位置。
  async function locateReferencedMessage(conversation, messageID) {
    var sequence = ++referenceNavigationSeq;
    followingMessages = false;
    var errorElement = $("cv-conversation-error");
    errorElement.hidden = true;
    try {
      while (!conversation.messageIDs[messageID] && conversation.before) {
        var result = await requestWebsiteJSON(
          "/api/public/website-channels/" + encodeURIComponent(channelID) +
          "/conversations/" + encodeURIComponent(conversation.id) +
          "/messages?before=" + encodeURIComponent(conversation.before),
        );
        if (sequence !== referenceNavigationSeq) {
          return;
        }
        var previousHeight = messages.scrollHeight;
        var previousTop = messages.scrollTop;
        CerviMarkdown.renderBatch(function () {
          result.messages.forEach(function (value) {
            appendServerMessage(conversation, value);
          });
        });
        conversation.before = result.before || "";
        messages.scrollTop = previousTop + messages.scrollHeight - previousHeight;
      }
      if (sequence !== referenceNavigationSeq) {
        return;
      }
      var target = messages.querySelector('[data-message-id="' + messageID + '"]');
      if (!target) {
        throw new Error(referenceLabels.unavailable);
      }
      messages.querySelectorAll(".cv-message-highlight").forEach(function (node) {
        node.classList.remove("cv-message-highlight");
      });
      target.scrollIntoView({ block: "center" });
      target.focus({ preventScroll: true });
      target.classList.add("cv-message-highlight");
      window.setTimeout(function () {
        target.classList.remove("cv-message-highlight");
      }, 2000);
    } catch (error) {
      if (sequence === referenceNavigationSeq) {
        errorElement.textContent = error.message || requestFailedLabel;
        errorElement.hidden = false;
      }
    }
  }

  // 返回当前渠道身份是否已在服务端建立。
  function hasVisitorIdentity() {
    return conversationItems.length > 0;
  }

  // 打开访客实时事件流，事件按行交给 onFrame，流结束交给 onClosed；返回幂等的关闭函数。
  function openVisitorEventStream(onFrame, onClosed) {
    var controller = new AbortController();
    var finished = false;
    var idleTimer = null;

    function finish(error) {
      if (finished) {
        return;
      }
      finished = true;
      window.clearTimeout(idleTimer);
      controller.abort();
      onClosed(error);
    }

    // 服务端每 25 秒发送心跳，超过空闲时限未收到数据按网络错误结束。
    function refreshIdle() {
      window.clearTimeout(idleTimer);
      idleTimer = window.setTimeout(function () {
        finish(new Error("visitor event stream idle timeout"));
      }, REALTIME_IDLE_TIMEOUT);
    }

    var headers = {
      Accept: "text/event-stream",
      "Accept-Language": document.documentElement.lang,
    };
    if (visitorToken) {
      headers["X-Cervi-Visitor-Token"] = visitorToken;
    }
    refreshIdle();
    window
      .fetch(
        "/api/public/website-channels/" +
          encodeURIComponent(channelID) +
          "/realtime",
        { headers: headers, signal: controller.signal, cache: "no-store" },
      )
      .then(function (response) {
        if (!response.ok || !response.body) {
          // 渠道、身份与访客 Token 错误重试不会改变结果，只有服务不可用按退避重连。
          var rejected = new Error("visitor event stream rejected");
          rejected.retryable = response.status >= 500;
          finish(rejected);
          return;
        }
        var reader = response.body.getReader();
        var decoder = new TextDecoder();
        var buffer = "";
        function read() {
          reader
            .read()
            .then(function (result) {
              if (finished) {
                return;
              }
              if (result.done) {
                finish();
                return;
              }
              refreshIdle();
              // 按行切分并保留跨数据块的半行；服务端每个事件只有一行 data，空行不处理。
              var lines = (
                buffer + decoder.decode(result.value, { stream: true })
              ).split("\n");
              buffer = lines.pop();
              lines.forEach(function (line) {
                if (finished) {
                  return;
                }
                var text =
                  line.charAt(line.length - 1) === "\r"
                    ? line.slice(0, -1)
                    : line;
                if (text.indexOf("data: ") === 0) {
                  onFrame(text.slice(6));
                }
              });
              read();
            })
            .catch(finish);
        }
        read();
      })
      .catch(finish);

    return function () {
      if (finished) {
        return;
      }
      finished = true;
      window.clearTimeout(idleTimer);
      controller.abort();
    };
  }

  // 按当前身份建立访客实时事件流；预览模式、尚无身份和已停止时不连接。
  function syncVisitorRealtime() {
    if (
      previewMode ||
      !initialized ||
      !hasVisitorIdentity() ||
      realtimeState !== "idle"
    ) {
      return;
    }
    connectVisitorRealtime();
  }

  // 发起一次事件流连接，只接收属于本次尝试的回调。
  function connectVisitorRealtime() {
    realtimeAttempt += 1;
    var attempt = realtimeAttempt;
    realtimeState = "connecting";
    realtimeClose = openVisitorEventStream(
      function (text) {
        if (attempt === realtimeAttempt) {
          receiveVisitorFrame(text);
        }
      },
      function (error) {
        if (attempt === realtimeAttempt) {
          closeVisitorRealtime(error);
        }
      },
    );
  }

  // 使当前连接尝试失效，关闭事件流并取消等待中的重连。
  function haltVisitorRealtime(state) {
    realtimeAttempt += 1;
    window.clearTimeout(realtimeTimer);
    realtimeTimer = null;
    clearRefreshRetry();
    var close = realtimeClose;
    realtimeClose = null;
    if (close) {
      close();
    }
    realtimeState = state;
  }

  // 解码并处理一条访客实时事件。
  function receiveVisitorFrame(text) {
    var event = null;
    try {
      event = JSON.parse(text);
    } catch (error) {
      console.warn("忽略无法解析的访客实时事件", error);
      return;
    }
    if (!event || typeof event.type !== "string") {
      return;
    }
    if (event.v !== REALTIME_PROTOCOL_VERSION) {
      console.warn("访客实时事件流协议主版本不受支持，停止重连", {
        version: event.v,
      });
      haltVisitorRealtime("stopped");
      return;
    }
    if (event.type === "visitor_hello") {
      realtimeFailures = 0;
      refreshFailures = 0;
      realtimeState = "ready";
      // 事件流建立之前提交的变更经重新拉取目录与当前线程窗口收敛。
      clearRefreshRetry();
      refreshConversationDirectory();
      refreshActiveConversationMessages();
      return;
    }
    if (event.type === "conversation_changed") {
      applyVisitorConversationChanged(event.data ? event.data.conversationId : "");
      return;
    }
    if (event.type === "visitor_typing") {
      var typing = event.data || {};
      var target = conversationByID[typing.conversationId];
      if (target) {
        applyConversationTyping(target, typing.active === true);
      }
    }
  }

  // 处理事件流结束：不可重试的拒绝停止重连，其余按抖动退避重连。
  function closeVisitorRealtime(error) {
    realtimeClose = null;
    if (error && error.retryable === false) {
      console.warn("访客实时事件流被拒绝，停止重连", error);
      haltVisitorRealtime("stopped");
      return;
    }
    realtimeFailures += 1;
    var ceiling = Math.min(
      REALTIME_BACKOFF_MAX,
      REALTIME_BACKOFF_BASE * Math.pow(2, realtimeFailures - 1),
    );
    // 等待时间取上限的一半到上限之间，多个挂件同时断开时错开重连。
    var delay = ceiling / 2 + (Math.random() * ceiling) / 2;
    realtimeState = "backoff";
    realtimeTimer = window.setTimeout(function () {
      realtimeTimer = null;
      connectVisitorRealtime();
    }, delay);
  }

  // 按会话变更通知重新拉取目录，并补拉当前打开线程的增量消息。
  function applyVisitorConversationChanged(conversationID) {
    refreshConversationDirectory();
    var conversation = conversationID ? conversationByID[conversationID] : null;
    if (!conversation) {
      return;
    }
    if (
      conversation.historyLoading ||
      (conversation === activeConversation && conversation.historyLoaded)
    ) {
      refreshConversationMessages(conversation);
    }
  }

  // 页面回到前台或挂件重新显示时跳过剩余退避并重新拉取。
  function handleVisitorForeground() {
    if (
      previewMode ||
      !messengerVisible ||
      document.visibilityState !== "visible"
    ) {
      return;
    }
    if (realtimeState === "backoff") {
      window.clearTimeout(realtimeTimer);
      realtimeTimer = null;
      connectVisitorRealtime();
      return;
    }
    if (realtimeState !== "ready") {
      syncVisitorRealtime();
      return;
    }
    clearRefreshRetry();
    refreshConversationDirectory();
    refreshActiveConversationMessages();
  }

  // 取消待重试的拉取；调用方随即执行的完整拉取取代它。
  function clearRefreshRetry() {
    window.clearTimeout(refreshRetryTimer);
    refreshRetryTimer = null;
  }

  // 一次拉取成功且没有待重试时结束退避计数。
  function noteRefreshSuccess() {
    if (refreshRetryTimer === null) {
      refreshFailures = 0;
    }
  }

  // 拉取失败后按抖动退避重试，直到一次成功；断网期间到达的通知据此收敛，事件流停止后不再重试。
  function scheduleRefreshRetry() {
    if (refreshRetryTimer !== null || realtimeState === "stopped") {
      return;
    }
    refreshFailures += 1;
    var ceiling = Math.min(
      REALTIME_BACKOFF_MAX,
      REALTIME_BACKOFF_BASE * Math.pow(2, refreshFailures - 1),
    );
    var delay = ceiling / 2 + (Math.random() * ceiling) / 2;
    refreshRetryTimer = window.setTimeout(function () {
      refreshRetryTimer = null;
      if (realtimeState === "stopped") {
        return;
      }
      refreshConversationDirectory();
      refreshActiveConversationMessages();
    }, delay);
  }

  // 重新拉取访客线程目录，发现新线程并更新既有摘要；在途时结束后补拉一次。
  function refreshConversationDirectory() {
    if (previewMode || !initialized) {
      return;
    }
    if (directoryRefreshing) {
      directoryRefreshQueued = true;
      return;
    }
    directoryRefreshing = true;
    requestWebsiteJSON(
      "/api/public/website-channels/" +
        encodeURIComponent(channelID) +
        "/conversations",
    )
      .then(function (result) {
        noteRefreshSuccess();
        result.conversations.forEach(function (summary) {
          upsertRealConversation(summary, null);
        });
        renderRecentConversation();
      })
      .catch(function (error) {
        console.warn("拉取网站访客线程目录失败", error);
        scheduleRefreshRetry();
      })
      .finally(function () {
        directoryRefreshing = false;
        if (directoryRefreshQueued) {
          directoryRefreshQueued = false;
          refreshConversationDirectory();
        }
      });
  }

  // 补拉当前打开线程的增量消息。
  function refreshActiveConversationMessages() {
    if (
      previewMode ||
      !activeConversation.id ||
      (!activeConversation.historyLoaded && !activeConversation.historyLoading)
    ) {
      return;
    }
    refreshConversationMessages(activeConversation);
  }

  // 使用服务端 after 游标增量补拉指定访客会话；历史加载或在途时登记，结束后补读。
  function refreshConversationMessages(conversation) {
    if (conversation.historyLoading || conversation.refreshing) {
      conversation.refreshPending = true;
      return;
    }
    var requestAfter = conversation.after;
    var requestSeq = conversation.refreshSeq;
    var path =
      "/api/public/website-channels/" +
      encodeURIComponent(channelID) +
      "/conversations/" +
      encodeURIComponent(conversation.id) +
      "/messages";
    if (requestAfter) {
      path += "?after=" + encodeURIComponent(requestAfter);
    }
    conversation.refreshing = true;
    requestWebsiteJSON(path)
      .then(function (result) {
        noteRefreshSuccess();
        if (
          conversation.refreshSeq !== requestSeq ||
          (requestAfter && conversation.after !== requestAfter)
        ) {
          return;
        }
        var followLatest = conversation === activeConversation && followingMessages;
        result.messages.forEach(function (message) {
          appendServerMessage(conversation, message);
        });
        // 评价和周期重开不产生访客可见消息，每次拉取都按服务端评价状态收敛已渲染的结束事件。
        syncSessionRatings(conversation, result.sessionRatings);
        if (followLatest) {
          scrollToBottom();
        }
        if (result.messages.length === 0) {
          return;
        }
        // 服务端按页返回，本页取满时继续沿游标补拉到线程尾端。
        conversation.refreshPending = true;
        if (result.after) {
          conversation.after = result.after;
        }
        var lastMessage = lastDialogueMessage(result.messages);
        if (!lastMessage) {
          return;
        }
        updateConversationSummary(
          conversation,
          messagePreview(lastMessage),
          lastMessage.originatedAt,
          lastMessage.messageSeq,
        );
      })
      .catch(function (error) {
        console.warn("拉取网站访客会话消息失败", {
          conversationId: conversation.id,
          error: error,
        });
        scheduleRefreshRetry();
      })
      .finally(function () {
        conversation.refreshing = false;
        if (conversation.refreshPending) {
          conversation.refreshPending = false;
          refreshConversationMessages(conversation);
        }
      });
  }

  // 生成客户端消息编号。
  function createClientMessageID() {
    return window.crypto.randomUUID();
  }

  // 发送真实网站访客文本消息。
  function sendRealMessage(text) {
    if (
      !initialized ||
      messageRequestPending ||
      activeConversation.historyLoading ||
      activeConversation.creating !== null
    ) {
      return;
    }
    var conversation = activeConversation;
    var startsConversation = conversation.id === null;
    // 首条消息由文本或附件先到者创建会话，另一路等待会话编号后复用。
    var creatingConversation = conversation;
    var settleCreation = null;
    if (startsConversation) {
      conversation.creating = new Promise(function (resolve) {
        settleCreation = resolve;
      });
    }
    var replyToID = conversation.replyTo ? conversation.replyTo.id : "";
    conversation.refreshSeq += 1;
    if (
      conversation.pendingBody !== text ||
      conversation.pendingReplyToID !== replyToID ||
      !conversation.pendingMessageID
    ) {
      conversation.pendingReplyToID = replyToID;
      conversation.pendingBody = text;
      conversation.pendingMessageID = createClientMessageID();
    }
    messageRequestPending = true;
    $("cv-conversation-error").hidden = true;
    updateSendState();
    requestWebsiteJSON(
      "/api/public/website-channels/" +
        encodeURIComponent(channelID) +
        "/messages",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          clientMessageId: conversation.pendingMessageID,
          replyToMessageId: replyToID,
          conversationId: conversation.id,
          body: text,
        }),
      },
    )
      .then(function (result) {
        conversation = upsertRealConversation(
          result.conversation,
          conversation,
        );
        conversation.pendingMessageID = "";
        conversation.pendingBody = "";
        // 只清除本次成功发送的草稿，保留等待期间修改的正文或引用。
        var currentDraft = conversation === activeConversation ? input.value : conversation.draft;
        var currentReplyToID = conversation.replyTo ? conversation.replyTo.id : "";
        var sentDraft = currentDraft.trim() === text && currentReplyToID === replyToID;
        conversation.draft = sentDraft ? "" : currentDraft;
        if (sentDraft) {
          conversation.replyTo = null;
        }
        appendServerMessage(conversation, result.message);
        if (startsConversation) {
          conversation.historyLoaded = true;
        } else if (!conversation.historyLoaded) {
          loadConversationHistory(conversation);
        }
        if (conversation === activeConversation) {
          if (sentDraft) {
            input.value = "";
          }
          intro.hidden = true;
          renderComposerReference();
          referenceNavigationSeq += 1;
          scrollToBottom();
          autosize();
        }
        renderRecentConversation();
        // 首条消息建立渠道身份后开始接收实时事件。
        syncVisitorRealtime();
      })
      .catch(function (error) {
        if (conversation === activeConversation) {
          $("cv-conversation-error").textContent =
            error.message || requestFailedLabel;
          $("cv-conversation-error").hidden = false;
        }
      })
      .finally(function () {
        messageRequestPending = false;
        if (settleCreation) {
          creatingConversation.creating = null;
          settleCreation();
        }
        updateSendState();
        if (conversation === activeConversation) {
          input.focus();
        }
      });
  }

  function insertEmoji(emoji) {
    var start =
      input.selectionStart === null ? input.value.length : input.selectionStart;
    var end =
      input.selectionEnd === null ? input.value.length : input.selectionEnd;
    input.value = input.value.slice(0, start) + emoji + input.value.slice(end);
    var cursor = start + emoji.length;
    input.setSelectionRange(cursor, cursor);
    input.focus();
    autosize();
    updateSendState();
    reportTypingInput();
  }

  function fillEmojiPanel() {
    emojiPanel.innerHTML = "";
    emojis.forEach(function (emoji) {
      var button = document.createElement("button");
      button.type = "button";
      button.textContent = emoji;
      button.setAttribute("aria-label", emoji);
      button.addEventListener("click", function () {
        insertEmoji(emoji);
        closeOverlays();
      });
      emojiPanel.appendChild(button);
    });
  }

  function fileKind(contentType) {
    contentType = contentType || "";
    if (contentType.indexOf("image/") === 0) {
      return "image";
    }
    if (contentType.indexOf("video/") === 0) {
      return "video";
    }
    if (contentType.indexOf("audio/") === 0) {
      return "audio";
    }
    return "file";
  }

  function listFiles(fileList) {
    var files = [];
    var limit = Math.min(fileList.length, MAX_ATTACHMENT_COUNT);
    for (var index = 0; index < limit; index += 1) {
      files.push(fileList[index]);
    }
    return files;
  }

  function addFiles(fileList) {
    var files = listFiles(fileList);
    if (files.length === 0) {
      return;
    }
    if (previewMode) {
      appendVisitorMessage("", files);
      scheduleDemoReply();
      input.focus();
      return;
    }
    if (!initialized || activeConversation.historyLoading) {
      return;
    }
    var conversation = activeConversation;
    // 正文和引用正在作为文本消息发送时由该次发送持有，不重复挂到附件上。
    var consumesDraft = !messageRequestPending;
    var body = consumesDraft ? input.value.trim() : "";
    var replyTo = consumesDraft ? conversation.replyTo : null;
    // 每个文件各自成为一条消息，说明随最后一个发送，引用只挂在第一个上。
    files.forEach(function (file, index) {
      conversation.pendingAttachments.push(
        createPendingAttachment(conversation, file, {
          body: index === files.length - 1 ? body : "",
          replyTo: index === 0 ? replyTo : null,
        }),
      );
    });
    startConversationIntro(conversation);
    if (consumesDraft) {
      input.value = "";
      conversation.draft = "";
      conversation.replyTo = null;
      renderComposerReference();
      autosize();
    }
    updateSendState();
    input.focus();
    processAttachmentQueue(conversation);
  }

  // 首个附件发出后隐藏会话引导区。
  function startConversationIntro(conversation) {
    if (conversation === activeConversation) {
      intro.hidden = true;
    }
  }

  // 创建待发送附件及其本地气泡。
  function createPendingAttachment(conversation, file, options) {
    var entry = {
      file: file,
      body: options.body,
      replyTo: options.replyTo,
      replyToID: options.replyTo ? options.replyTo.id : "",
      clientMessageID: createClientMessageID(),
      fileID: "",
      state: "pending",
      progress: 0,
      canceled: false,
      request: null,
      error: "",
      node: messageContainer("visitor", new Date(), { key: "self" }),
      status: document.createElement("span"),
      action: document.createElement("button"),
    };
    var row = document.createElement("div");
    row.className = "cv-message-row cv-message-row-with-assets";
    if (entry.body || entry.replyTo) {
      var bubble = document.createElement("div");
      bubble.className = "cv-message-bubble";
      if (entry.replyTo) {
        var reference = document.createElement("blockquote");
        reference.className = "cv-message-reference";
        var author = document.createElement("strong");
        author.textContent = referenceLabels[entry.replyTo.author];
        var excerpt = document.createElement("span");
        excerpt.className = "cv-message-reference-body";
        excerpt.textContent = CerviMarkdown.preview(entry.replyTo.body, entry.replyTo.senderIdentityType);
        reference.appendChild(author);
        reference.appendChild(excerpt);
        bubble.appendChild(reference);
      }
      if (entry.body) {
        var paragraph = document.createElement("div");
        paragraph.textContent = entry.body;
        bubble.appendChild(bubbleContent(paragraph, new Date()));
      }
      row.appendChild(bubble);
    }
    row.appendChild(assetList([file]));
    entry.node.appendChild(row);
    var meta = document.createElement("div");
    meta.className = "cv-message-meta cv-attachment-meta";
    entry.status.className = "cv-attachment-status";
    entry.action.type = "button";
    entry.action.className = "cv-attachment-action";
    entry.action.addEventListener("click", function () {
      if (entry.state !== "failed") {
        cancelPendingAttachment(conversation, entry);
        return;
      }
      entry.state = "pending";
      entry.error = "";
      renderAttachmentStatus(entry);
      processAttachmentQueue(conversation);
    });
    meta.appendChild(entry.status);
    meta.appendChild(entry.action);
    entry.node.appendChild(meta);
    appendConversationNode(conversation, entry.node);
    renderAttachmentStatus(entry);
    return entry;
  }

  // 同步待发送附件的状态文字和操作。
  function renderAttachmentStatus(entry) {
    entry.action.disabled = false;
    if (entry.state === "failed") {
      entry.status.textContent = entry.error || attachmentLabels.failed;
      entry.action.textContent = attachmentLabels.retry;
      return;
    }
    entry.status.textContent =
      entry.progress > 0 && entry.progress < 100
        ? attachmentLabels.uploading + " " + entry.progress + "%"
        : attachmentLabels.uploading;
    entry.action.textContent = attachmentLabels.cancel;
  }

  // 取消待发送附件并移除本地气泡，未关联的临时文件由服务端按过期清理。
  function cancelPendingAttachment(conversation, entry) {
    entry.canceled = true;
    if (entry.request) {
      entry.request.abort();
    }
    var index = conversation.pendingAttachments.indexOf(entry);
    if (index >= 0) {
      conversation.pendingAttachments.splice(index, 1);
    }
    entry.node.remove();
  }

  // 按选择顺序上传并发送当前会话的待发送附件。
  async function processAttachmentQueue(conversation) {
    if (conversation.attachmentSending) {
      return;
    }
    conversation.attachmentSending = true;
    try {
      while (true) {
        var entry = conversation.pendingAttachments.find(function (item) {
          return item.state === "pending";
        });
        if (!entry) {
          return;
        }
        entry.state = "uploading";
        entry.progress = 0;
        renderAttachmentStatus(entry);
        var target = conversation;
        try {
          target = await sendPendingAttachment(conversation, entry);
        } catch (error) {
          if (entry.canceled) {
            continue;
          }
          entry.state = "failed";
          entry.error = error.message || requestFailedLabel;
          renderAttachmentStatus(entry);
          continue;
        }
        if (target !== conversation) {
          // 线程编号已由目录刷新建立占位对象时，剩余附件跟随接管后的会话继续发送。
          target.pendingAttachments = target.pendingAttachments.concat(conversation.pendingAttachments);
          conversation.pendingAttachments = [];
          processAttachmentQueue(target);
          return;
        }
      }
    } finally {
      conversation.attachmentSending = false;
    }
  }

  // 完成一个附件的上传与发送，返回承载该消息的会话。
  async function sendPendingAttachment(conversation, entry) {
    var attachmentsPath =
      "/api/public/website-channels/" + encodeURIComponent(channelID) + "/attachments";
    if (!entry.fileID) {
      var upload = await requestWebsiteJSON(attachmentsPath, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          fileName: entry.file.name,
          contentType: entry.file.type || "application/octet-stream",
          byteSize: entry.file.size,
        }),
      });
      if (entry.canceled) {
        return conversation;
      }
      await uploadAttachmentContent(entry, upload.request);
      await requestWebsiteJSON(
        attachmentsPath + "/" + encodeURIComponent(upload.fileId),
        { method: "POST" },
      );
      entry.fileID = upload.fileId;
    }
    var size = await readImageSize(entry.file);
    if (entry.canceled) {
      return conversation;
    }
    // 首条消息由文本或附件先到者创建会话，另一路等待会话编号后复用。
    if (conversation.creating) {
      await conversation.creating;
    }
    var startsConversation = conversation.id === null;
    var settleCreation = null;
    if (startsConversation) {
      conversation.creating = new Promise(function (resolve) {
        settleCreation = resolve;
      });
      updateSendState();
    }
    // 消息提交后无法撤回，提交期间关闭取消入口。
    entry.action.disabled = true;
    var result;
    try {
      result = await requestWebsiteJSON(
        "/api/public/website-channels/" + encodeURIComponent(channelID) + "/attachment-messages",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            clientMessageId: entry.clientMessageID,
            replyToMessageId: entry.replyToID,
            conversationId: conversation.id,
            fileId: entry.fileID,
            body: entry.body,
            imageWidth: size.width,
            imageHeight: size.height,
          }),
        },
      );
    } finally {
      if (settleCreation) {
        conversation.creating = null;
        settleCreation();
      }
      updateSendState();
    }
    var target = upsertRealConversation(result.conversation, conversation);
    var index = conversation.pendingAttachments.indexOf(entry);
    if (index >= 0) {
      conversation.pendingAttachments.splice(index, 1);
    }
    entry.node.remove();
    appendServerMessage(target, result.message);
    if (startsConversation) {
      target.historyLoaded = true;
    } else if (!target.historyLoaded) {
      loadConversationHistory(target);
    }
    if (target === activeConversation) {
      intro.hidden = true;
      scrollToBottom();
    }
    renderRecentConversation();
    // 首条消息建立渠道身份后开始接收实时事件。
    syncVisitorRealtime();
    return target;
  }

  // 以可取消并带进度的请求直传附件内容。
  function uploadAttachmentContent(entry, request) {
    return new Promise(function (resolve, reject) {
      var transfer = new XMLHttpRequest();
      entry.request = transfer;
      transfer.open(request.method, request.url, true);
      Object.keys(request.headers || {}).forEach(function (name) {
        transfer.setRequestHeader(name, request.headers[name]);
      });
      transfer.upload.addEventListener("progress", function (event) {
        if (!event.lengthComputable) {
          return;
        }
        entry.progress = Math.round((event.loaded / event.total) * 100);
        renderAttachmentStatus(entry);
      });
      transfer.addEventListener("load", function () {
        entry.request = null;
        if (transfer.status >= 200 && transfer.status < 300) {
          resolve();
          return;
        }
        reject(new Error(requestFailedLabel));
      });
      transfer.addEventListener("error", function () {
        entry.request = null;
        reject(new Error(requestFailedLabel));
      });
      transfer.addEventListener("abort", function () {
        entry.request = null;
        reject(new Error(requestFailedLabel));
      });
      transfer.send(entry.file);
    });
  }

  // 读取图片附件的像素尺寸，非图片或无法解码时为 0。
  function readImageSize(file) {
    return new Promise(function (resolve) {
      if (fileKind(file.type) !== "image") {
        resolve({ width: 0, height: 0 });
        return;
      }
      var url = URL.createObjectURL(file);
      var image = new Image();
      image.addEventListener("load", function () {
        resolve({ width: image.naturalWidth, height: image.naturalHeight });
        URL.revokeObjectURL(url);
      });
      image.addEventListener("error", function () {
        resolve({ width: 0, height: 0 });
        URL.revokeObjectURL(url);
      });
      image.src = url;
    });
  }

  function assetList(files) {
    var assets = document.createElement("div");
    assets.className = "cv-message-assets";
    files.forEach(function (file) {
      assets.appendChild(mediaNode(file));
    });
    return assets;
  }

  function mediaNode(file) {
    var kind = fileKind(file.type);
    var url = URL.createObjectURL(file);
    if (kind === "image") {
      var imageButton = document.createElement("button");
      imageButton.type = "button";
      imageButton.className = "cv-asset";
      var image = document.createElement("img");
      image.src = url;
      image.alt = file.name;
      imageButton.appendChild(image);
      imageButton.addEventListener("click", function () {
        openLightbox(url, file.name);
      });
      return imageButton;
    }
    if (kind === "video") {
      var videoWrap = document.createElement("div");
      videoWrap.className = "cv-asset";
      var video = document.createElement("video");
      video.src = url;
      video.controls = true;
      video.playsInline = true;
      videoWrap.appendChild(video);
      return videoWrap;
    }
    if (kind === "audio") {
      var audioWrap = document.createElement("div");
      audioWrap.className = "cv-asset cv-file-asset";
      var audio = document.createElement("audio");
      audio.src = url;
      audio.controls = true;
      audioWrap.appendChild(audio);
      return audioWrap;
    }
    var fileLink = document.createElement("a");
    fileLink.className = "cv-asset cv-file-asset";
    fileLink.href = url;
    fileLink.download = file.name;
    var name = document.createElement("strong");
    name.textContent = file.name;
    var size = document.createElement("span");
    size.textContent = formatSize(file.size);
    fileLink.appendChild(name);
    fileLink.appendChild(size);
    return fileLink;
  }

  function openLightbox(url, name) {
    if (!lightbox) {
      lightbox = document.createElement("button");
      lightbox.type = "button";
      lightbox.className = "cv-lightbox";
      lightbox.addEventListener("click", function () {
        lightbox.hidden = true;
      });
      document.body.appendChild(lightbox);
    }
    lightbox.innerHTML = "";
    var image = document.createElement("img");
    image.src = url;
    image.alt = name;
    lightbox.appendChild(image);
    lightbox.hidden = false;
  }

  function pastedImageFiles(event) {
    var clipboard = event.clipboardData;
    if (!clipboard) {
      return [];
    }
    var files = [];
    for (var index = 0; index < clipboard.items.length; index += 1) {
      var item = clipboard.items[index];
      if (item.kind !== "file" || item.type.indexOf("image/") !== 0) {
        continue;
      }
      var file = item.getAsFile();
      if (file) {
        files.push(file);
      }
    }
    return files;
  }

  function startRecording() {
    if (!previewMode) {
      return;
    }
    closeOverlays();
    composerMain.hidden = true;
    recording.hidden = false;
    recordingStartedAt = Date.now();
    recordTime.textContent = "0:00";
    recordingTimer = window.setInterval(function () {
      recordTime.textContent = formatDuration(
        Math.floor((Date.now() - recordingStartedAt) / 1000),
      );
    }, 250);
  }

  function resetRecording(restoreFocus) {
    if (recordingTimer !== null) {
      window.clearInterval(recordingTimer);
      recordingTimer = null;
    }
    recordingStartedAt = 0;
    recording.hidden = true;
    composerMain.hidden = false;
    recordTime.textContent = "0:00";
    if (restoreFocus !== false && activeRoute === "conversation") {
      input.focus();
    }
  }

  function stopRecording() {
    var duration = Math.max(
      1,
      Math.floor((Date.now() - recordingStartedAt) / 1000),
    );
    resetRecording();
    startConversation();
    var now = new Date();
    var message = messageContainer("visitor", now, { key: "self" });
    var row = document.createElement("div");
    row.className = "cv-message-row";
    var bubble = document.createElement("div");
    bubble.className = "cv-message-bubble cv-voice-message";
    var play = document.createElement("button");
    play.type = "button";
    play.className = "cv-voice-play";
    play.setAttribute("aria-label", playVoiceLabel);
    play.setAttribute("title", playVoiceLabel);
    play.setAttribute("aria-pressed", "false");
    var line = document.createElement("span");
    line.className = "cv-voice-line";
    var time = document.createElement("span");
    time.className = "cv-voice-duration";
    time.textContent = formatDuration(duration);
    var playbackTimer = null;
    play.style.setProperty("--cv-voice-duration", duration + "s");
    play.addEventListener("click", function () {
      var playing = play.getAttribute("data-playing") === "true";
      if (playbackTimer !== null) {
        window.clearTimeout(playbackTimer);
        playbackTimer = null;
      }
      play.setAttribute("data-playing", String(!playing));
      play.setAttribute("aria-pressed", String(!playing));
      play.setAttribute(
        "aria-label",
        playing ? playVoiceLabel : pauseVoiceLabel,
      );
      play.setAttribute("title", playing ? playVoiceLabel : pauseVoiceLabel);
      if (!playing) {
        playbackTimer = window.setTimeout(function () {
          play.setAttribute("data-playing", "false");
          play.setAttribute("aria-pressed", "false");
          play.setAttribute("aria-label", playVoiceLabel);
          play.setAttribute("title", playVoiceLabel);
          playbackTimer = null;
        }, duration * 1000);
      }
    });
    bubble.appendChild(play);
    bubble.appendChild(line);
    bubble.appendChild(time);
    bubble.appendChild(messageTime(now));
    row.appendChild(bubble);
    message.appendChild(row);
    messages.appendChild(message);
    updateConversationSummary(
      activeConversation,
      formatDuration(duration),
      now,
    );
    scrollToBottom();
    scheduleDemoReply();
  }

  function relativeLuminance(hexColor) {
    var channels = hexColor
      .slice(1)
      .match(/.{2}/g)
      .map(function (channel) {
        var value = Number.parseInt(channel, 16) / 255;
        return value <= 0.04045
          ? value / 12.92
          : Math.pow((value + 0.055) / 1.055, 2.4);
      });
    return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  }

  function forEachConversationNode(selector, callback) {
    document.querySelectorAll(selector).forEach(callback);
    conversationItems.forEach(function (conversation) {
      if (conversation !== activeConversation) {
        conversation.fragment.querySelectorAll(selector).forEach(callback);
      }
    });
  }

  function applyPreviewValue(value) {
    var title = typeof value.title === "string" ? value.title.trim() : "";
    var subtitle =
      typeof value.subtitle === "string" ? value.subtitle.trim() : "";
    var greeting =
      typeof value.greetingMessage === "string"
        ? value.greetingMessage.trim()
        : "";
    var themeColor =
      typeof value.themeColor === "string"
        ? value.themeColor.trim().toUpperCase()
        : "";
    title = title || defaultTitle;
    document.title = title;
    displayTitle = title;
    forEachConversationNode("[data-channel-title]", function (node) {
      node.textContent = title;
    });
    forEachConversationNode("[data-channel-subtitle]", function (node) {
      node.textContent = subtitle || defaultSubtitle;
    });
    forEachConversationNode("[data-channel-greeting]", function (node) {
      node.textContent = greeting || defaultGreeting;
    });
    forEachConversationNode(
      '[data-greeting="true"] .cv-message-text',
      function (node) {
        node.textContent = greeting || defaultGreeting;
      },
    );
    if (/^#[0-9A-F]{6}$/.test(themeColor)) {
      var luminance = relativeLuminance(themeColor);
      var whiteContrast = 1.05 / (luminance + 0.05);
      var darkContrast =
        (luminance + 0.05) / (relativeLuminance("#1C1917") + 0.05);
      var focus =
        whiteContrast < 3
          ? "rgba(28, 25, 23, 0.35)"
          : "color-mix(in srgb, " + themeColor + " 40%, transparent)";
      document.documentElement.style.setProperty("--cv-theme", themeColor);
      document.documentElement.style.setProperty(
        "--cv-on-theme",
        whiteContrast >= darkContrast ? "#FFFFFF" : "#1C1917",
      );
      document.documentElement.style.setProperty("--cv-focus", focus);
    }
    renderRecentConversation();
  }

  function applyWidgetState(value) {
    messengerVisible = value.visible === true;
    expanded = value.expanded === true;
    var expandButton = $("cv-expand");
    if (expandButton) {
      expandButton.hidden = value.expandable === false;
      var label = expanded ? collapseWindowLabel : expandWindowLabel;
      var text = expandButton.querySelector("span");
      if (text) {
        text.textContent = label;
      }
      expandButton.setAttribute("aria-pressed", String(expanded));
    }
    syncMoreAvailability();
    if (!messengerVisible && !recording.hidden) {
      resetRecording(false);
    }
    if (messengerVisible && activeRoute === "conversation") {
      clearUnread();
    }
    autosize();
    handleVisitorForeground();
  }

  syncMoreAvailability();
  fillEmojiPanel();
  autosize();
  updateSendState();

  document.querySelectorAll("[data-route-target]").forEach(function (trigger) {
    trigger.addEventListener("click", function (event) {
      event.preventDefault();
      navigate(trigger.getAttribute("data-route-target"));
    });
  });
  document
    .querySelectorAll("[data-new-conversation]")
    .forEach(function (button) {
      button.addEventListener("click", beginNewConversation);
    });
  document
    .querySelectorAll("[data-resume-conversation]")
    .forEach(function (button) {
      button.addEventListener("click", resumeRecentConversation);
    });
  document.querySelectorAll("[data-help-topic]").forEach(function (button) {
    button.addEventListener("click", function () {
      showHelpTopic(button);
    });
  });
  document.querySelectorAll("[data-back-to]").forEach(function (button) {
    button.addEventListener("click", function () {
      var route = button.getAttribute("data-back-to");
      if (activeRoute === "conversation") {
        route = conversationReturnRoute;
      }
      navigate(route);
    });
  });
  document.querySelectorAll("[data-close]").forEach(function (button) {
    button.addEventListener("click", closeMessenger);
  });

  document.addEventListener(
    "mousedown",
    rememberNativeContextMenuGesture,
    true,
  );
  document.addEventListener(
    "keydown",
    rememberKeyboardContextMenuGesture,
    true,
  );
  document.addEventListener(
    "contextmenu",
    allowOnlyNativeSecondaryButtonMenu,
    true,
  );
  window.addEventListener("blur", function () {
    nativeContextMenuSource = "";
  });

  // 下一次独立按下时关闭菜单，保留长按结束后浏览器补发的点击。
  document.addEventListener("pointerdown", function (event) {
    if (!replyMenu.contains(event.target)) {
      replyMenu.hidden = true;
    }
  });
  document.addEventListener("click", function (event) {
    if (!replyMenu.hidden && replyMenuSource.contains(event.target)) {
      event.preventDefault();
      event.stopPropagation();
    }
  }, true);

  $("cv-menu-reply").addEventListener("click", function () {
    var value = activeConversation.messageIDs[replyMenuSource.getAttribute("data-message-id")];
    selectReplyMessage(value);
  });
  $("cv-cancel-reply").addEventListener("click", function () {
    activeConversation.replyTo = null;
    renderComposerReference();
    input.focus();
  });
  $("cv-latest-message").addEventListener("click", function () {
    referenceNavigationSeq += 1;
    scrollToBottom();
    input.focus();
  });
  messages.addEventListener("scroll", function () {
    replyMenu.hidden = true;
    $("cv-latest-message").hidden = previewMode ||
      messages.scrollHeight - messages.scrollTop - messages.clientHeight < 48;
  });
  window.addEventListener("resize", function () {
    replyMenu.hidden = true;
  });

  $("cv-help-input").addEventListener("input", filterHelp);
  input.addEventListener("input", function () {
    if (!previewMode && activeConversation.pendingBody !== input.value.trim()) {
      activeConversation.pendingBody = "";
      activeConversation.pendingMessageID = "";
    }
    autosize();
    updateSendState();
    reportTypingInput();
  });
  input.addEventListener("keydown", function (event) {
    if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
      return;
    }
    event.preventDefault();
    sendMessage();
  });
  input.addEventListener("paste", function (event) {
    var files = pastedImageFiles(event);
    if (files.length === 0) {
      return;
    }
    event.preventDefault();
    addFiles(files);
  });
  composer.addEventListener("submit", function (event) {
    event.preventDefault();
    sendMessage();
  });

  $("cv-attach").addEventListener("click", function () {
    fileInput.click();
  });
  fileInput.addEventListener("change", function () {
    addFiles(fileInput.files);
    fileInput.value = "";
  });
  $("cv-emoji-toggle").addEventListener("click", function () {
    var open = emojiPanel.hidden;
    closeOverlays();
    emojiPanel.hidden = !open;
    $("cv-emoji-toggle").setAttribute("aria-expanded", String(open));
  });
  $("cv-voice").addEventListener("click", startRecording);
  $("cv-initialization-retry").addEventListener(
    "click",
    initializeRealMessenger,
  );
  $("cv-home-initialization-retry").addEventListener(
    "click",
    initializeRealMessenger,
  );
  $("cv-record-cancel").addEventListener("click", function () {
    resetRecording();
  });
  $("cv-record-stop").addEventListener("click", stopRecording);

  moreToggle.addEventListener("click", function () {
    var open = moreMenu.hidden;
    closeOverlays();
    moreMenu.hidden = !open;
    moreToggle.setAttribute("aria-expanded", String(open));
  });
  if ($("cv-expand")) {
    $("cv-expand").addEventListener("click", function () {
      postToParent({ type: "cervi:toggle-expand" });
      closeOverlays();
    });
  }
  document.addEventListener("click", function (event) {
    if (
      emojiPanel.contains(event.target) ||
      $("cv-emoji-toggle").contains(event.target) ||
      moreMenu.contains(event.target) ||
      moreToggle.contains(event.target)
    ) {
      return;
    }
    closeOverlays();
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Tab") {
      replyMenu.hidden = true;
    }
    if (event.key !== "Escape") {
      return;
    }
    if (lightbox && !lightbox.hidden) {
      lightbox.hidden = true;
      return;
    }
    if (!replyMenu.hidden) {
      closeOverlays();
      replyMenuSource.focus({ preventScroll: true });
      return;
    }
    if (!emojiPanel.hidden || !moreMenu.hidden) {
      closeOverlays();
      return;
    }
    if (!recording.hidden) {
      resetRecording();
      return;
    }
    if (activeRoute === "conversation") {
      navigate(conversationReturnRoute);
      return;
    }
    if (activeRoute === "help-detail") {
      navigate("help");
      return;
    }
    if (
      document.documentElement.classList.contains("cv-embed") ||
      document.documentElement.classList.contains("cv-preview")
    ) {
      closeMessenger();
    }
  });

  window.addEventListener("message", function (event) {
    if (event.source !== window.parent) {
      return;
    }
    if (parentOrigin && event.origin !== parentOrigin) {
      return;
    }
    if (!event.data || typeof event.data.type !== "string") {
      return;
    }
    if (event.data.type === "cervi:widget-state") {
      applyWidgetState(event.data);
      postToParent({ type: "cervi:frame-ready" });
      return;
    }
    if (
      messenger.getAttribute("data-preview") === "true" &&
      event.data.type === "cervi:preview-config" &&
      event.data.value
    ) {
      applyPreviewValue(event.data.value);
      postToParent({ type: "cervi:preview-ready" });
    }
  });
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState !== "visible") {
      stopTypingReport();
    }
    handleVisitorForeground();
  });
  window.addEventListener("online", handleVisitorForeground);
  window.addEventListener("resize", autosize);
  if (!previewMode) {
    $("cv-voice").disabled = true;
    setNewConversationAvailability(false);
    initializeRealMessenger();
  }
})();
