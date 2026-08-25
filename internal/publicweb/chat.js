// 网站渠道访客 Messenger 交互。
(function () {
  var messenger = document.getElementById("cv-messenger");
  var messages = document.getElementById("cv-messages");
  var composer = document.getElementById("cv-composer");
  var input = document.getElementById("cv-input");

  var MAX_ATTACHMENT_COUNT = 10;
  var COMPOSER_MAX_HEIGHT = 160;
  var previewMode = messenger.getAttribute("data-preview") === "true";
  var channelID = messenger.getAttribute("data-channel-id");
  var visitorToken = "";
  var initialized = previewMode;
  var initializationPending = false;
  var messageRequestPending = false;
  var conversationItems = [];
  var conversationByID = Object.create(null);
  var activeRoute = "home";
  var conversationReturnRoute = "home";
  var activeConversation = createConversation();
  var recentConversation = null;
  var recordingStartedAt = 0;
  var recordingTimer = null;
  var lightbox = null;
  var parentOrigin = document.referrer ? new URL(document.referrer).origin : "";
  var messengerVisible = !document.documentElement.classList.contains("cv-embed");
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
  var requestFailedLabel = messenger.getAttribute("data-request-failed");
  var sessionLabels = {
  waiting: messenger.getAttribute("data-session-waiting"),
  active: messenger.getAttribute("data-session-active"),
  pending: messenger.getAttribute("data-session-pending"),
  closed: messenger.getAttribute("data-session-closed"),
  };
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
      var heading = document.querySelector('[data-screen="' + route + '"] [data-route-heading]');
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
    summary: summary ? summary.preview : "",
    time: summary ? formatTime(new Date(summary.lastMessageAt)) : "",
    lastMessageAt: summary ? summary.lastMessageAt : "",
    serviceSession: summary ? summary.serviceSession : null,
      unread: false,
      replyState: "none",
      typingNode: null,
    historyLoaded: false,
    historyLoading: false,
    messageIDs: Object.create(null),
    pendingMessageID: "",
    pendingBody: "",
    };
  }

  // 返回指定会话当前承载消息的容器。
  function conversationMessageContainer(conversation) {
    return conversation === activeConversation ? messages : conversation.fragment;
  }

  // 向指定会话追加节点，并只滚动当前会话。
  function appendConversationNode(conversation, node) {
    conversationMessageContainer(conversation).appendChild(node);
    if (conversation === activeConversation) {
      scrollToBottom();
    }
  }

  function stashActiveConversation() {
    activeConversation.draft = input.value;
    Array.from(messages.children).forEach(function (node) {
      if (node !== intro) {
        activeConversation.fragment.appendChild(node);
      }
    });
  }

  function showConversation(conversation) {
    if (activeConversation === conversation) {
    if (!previewMode && conversation.id && !conversation.historyLoaded && !conversation.historyLoading) {
    loadConversationHistory(conversation);
    }
      return;
    }
    stashActiveConversation();
    activeConversation = conversation;
    messages.appendChild(activeConversation.fragment);
  intro.hidden = previewMode ? activeConversation.started : activeConversation.id !== null;
    $("cv-conversation-error").hidden = true;
    input.value = activeConversation.draft;
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
    messages.scrollTop = activeConversation.started ? messages.scrollHeight : 0;
  if (!previewMode && activeConversation.id && !activeConversation.historyLoaded) {
    loadConversationHistory(activeConversation);
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

  // 打开指定的真实客户会话。
  function resumeConversation(conversation) {
  if (!conversation) {
    return;
  }
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
    var allow =
      nativeContextMenuSource !== "" ||
      (event.pointerType && event.pointerType !== "mouse");
    nativeContextMenuSource = "";
    if (!allow) {
      event.preventDefault();
    }
  }

  function showHelpTopic(button) {
    $("cv-help-detail-title").textContent = button.querySelector("strong").textContent;
    navigate("help-detail");
  }

  function filterHelp() {
    var query = $("cv-help-input").value.trim().toLocaleLowerCase();
    var visible = 0;
    document.querySelectorAll("#cv-collection-list [data-search-text]").forEach(function (button) {
      var searchText = button.getAttribute("data-search-text").toLocaleLowerCase();
      button.hidden = query !== "" && searchText.indexOf(query) === -1;
      if (!button.hidden) {
        visible += 1;
      }
    });
    $("cv-help-empty").hidden = visible !== 0;
  }

  function autosize() {
    input.style.height = "auto";
    input.style.height = Math.min(input.scrollHeight, COMPOSER_MAX_HEIGHT) + "px";
    input.style.overflowY = input.scrollHeight > COMPOSER_MAX_HEIGHT ? "auto" : "hidden";
  }

  function updateSendState() {
  sendButton.disabled =
    input.value.trim() === "" ||
    (!previewMode && (!initialized || messageRequestPending || activeConversation.historyLoading));
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

  // 把 UTC RFC3339 的小数秒补齐，避免字符串精度不同导致排序颠倒。
  function normalizedTimestamp(value) {
  var match = String(value || "").match(/^(.*?)(?:\.(\d+))?Z$/);
  if (!match) {
    return String(value || "");
  }
  return match[1] + "." + (match[2] || "").padEnd(9, "0").slice(0, 9) + "Z";
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
    messages.scrollTop = messages.scrollHeight;
  }

  function messageContainer(author) {
    var message = document.createElement("article");
    message.className = "cv-message cv-message-" + author;
    return message;
  }

  function messageMeta(date) {
    var meta = document.createElement("div");
    meta.className = "cv-message-meta";
    var time = document.createElement("time");
    time.dateTime = date.toISOString();
    time.textContent = formatTime(date);
    meta.appendChild(time);
    return meta;
  }

  function startConversation() {
    if (activeConversation.started) {
      return;
    }
    activeConversation.started = true;
    intro.hidden = true;
    var greeting = document.querySelector("[data-channel-greeting]").textContent.trim();
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
    var message = messageContainer("visitor");
    var row = document.createElement("div");
    row.className = "cv-message-row";
    if (text) {
      var bubble = document.createElement("div");
      bubble.className = "cv-message-bubble";
      var paragraph = document.createElement("div");
      paragraph.textContent = text;
      bubble.appendChild(paragraph);
      row.appendChild(bubble);
    }
    if (files.length > 0) {
      row.classList.add("cv-message-row-with-assets");
      row.appendChild(assetList(files));
    }
    message.appendChild(row);
    message.appendChild(messageMeta(now));
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
    var message = messageContainer("assistant");
    if (greeting) {
      message.setAttribute("data-greeting", "true");
    }
    var row = document.createElement("div");
    row.className = "cv-message-row";
    var bubble = document.createElement("div");
    bubble.className = "cv-message-bubble";
    bubble.textContent = text;
    row.appendChild(bubble);
    message.appendChild(row);
    message.appendChild(messageMeta(now));
    appendConversationNode(conversation, message);
    if (!greeting) {
      updateConversationSummary(conversation, text, now);
    }
  }

  // 向指定会话追加正在输入提示。
  function appendTyping(conversation) {
    var message = messageContainer("assistant");
    var typing = document.createElement("div");
    typing.className = "cv-typing";
    typing.innerHTML = "<i></i><i></i><i></i>";
    message.appendChild(typing);
    conversation.typingNode = message;
    appendConversationNode(conversation, message);
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
      if (conversation.typingNode) {
        conversation.typingNode.remove();
        conversation.typingNode = null;
      }
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
    $("cv-conversation-list").appendChild(conversationListButton(conversation));
  });
  $("cv-home-recent").querySelector("strong").textContent =
    previewMode ? displayTitle : recentConversation.title;
    $("cv-home-recent-preview").textContent = recentConversation.summary;
    $("cv-home-recent-time").textContent = recentConversation.time;
  unreadDot.hidden = previewMode ? !recentConversation.unread : true;
    $("cv-home-recent-unread-dot").hidden = !recentConversation.unread;
  postToParent({ type: "cervi:unread", unread: previewMode && recentConversation.unread });
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
  initials.textContent = document.querySelector(".cv-presence-avatar span").textContent;
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
    status.textContent = sessionLabels[conversation.serviceSession.status] || "";
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

  // 更新指定会话的摘要、时间和未读状态。
  function updateConversationSummary(conversation, preview, date) {
    conversation.summary = preview;
    conversation.time = formatTime(date);
  conversation.lastMessageAt = date.toISOString();
    conversation.unread =
      conversation !== activeConversation || activeRoute !== "conversation" || !messengerVisible;
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

  // 请求网站访客 JSON 接口。
  function requestWebsiteJSON(path, options) {
  options = options || {};
  options.credentials = "same-origin";
  options.headers = options.headers || {};
  options.headers.Accept = "application/json";
  options.headers["Accept-Language"] = document.documentElement.lang;
  if (visitorToken) {
    options.headers["X-Cervi-Visitor-Token"] = visitorToken;
  }
  return window.fetch(path, options).then(function (response) {
    return response.text().then(function (text) {
    var payload = null;
    if (text) {
      try {
      payload = JSON.parse(text);
      } catch (_error) {
      payload = null;
      }
    }
    if (!response.ok) {
      var message = payload && payload.error && payload.error.message;
      var error = new Error(message || requestFailedLabel);
      error.payload = payload;
      throw error;
    }
    return payload;
    });
  });
  }

  // 将服务端摘要合并到页面会话状态。
  function upsertRealConversation(summary, preferredConversation) {
  var conversation = conversationByID[summary.id];
  if (!conversation) {
    conversation = preferredConversation || createConversation(summary);
    conversation.id = summary.id;
    conversation.started = true;
    conversationByID[summary.id] = conversation;
    conversationItems.push(conversation);
  }
  conversation.title = summary.title;
  conversation.summary = summary.preview;
  conversation.lastMessageAt = summary.lastMessageAt;
  conversation.time = formatTime(new Date(summary.lastMessageAt));
  conversation.serviceSession = summary.serviceSession;
    conversationItems.sort(compareConversationRecency);
  if (!previewMode && conversationItems.length > 20) {
    var removed = conversationItems.pop();
    if (removed && removed !== conversation) {
    delete conversationByID[removed.id];
    }
  }
  recentConversation = conversationItems.length > 0 ? conversationItems[0] : null;
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
  requestWebsiteJSON("/api/public/website-channels/" + encodeURIComponent(channelID) + "/messenger")
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
  document.querySelectorAll("[data-new-conversation]").forEach(function (button) {
    button.disabled = !available;
  });
  }

  // 加载指定真实客户会话最近的持久消息。
  function loadConversationHistory(conversation) {
  if (conversation.historyLoading || !conversation.id) {
    return;
  }
  conversation.historyLoading = true;
  var requestMarker = {};
  conversation.historyRequest = requestMarker;
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
    if (conversation.historyRequest !== requestMarker) {
      return;
    }
    clearConversationMessages(conversation);
    result.messages.forEach(function (message) {
      appendServerMessage(conversation, message);
    });
    conversation.historyLoaded = true;
          if (conversation === activeConversation) {
            scrollToBottom();
          }
    })
    .catch(function (error) {
    if (conversation === activeConversation && conversation.historyRequest === requestMarker) {
      $("cv-conversation-error").textContent = error.message || requestFailedLabel;
      $("cv-conversation-error").hidden = false;
    }
    })
    .finally(function () {
    if (conversation.historyRequest !== requestMarker) {
      return;
    }
    conversation.historyLoading = false;
    if (conversation === activeConversation) {
      updateSendState();
    }
    });
  }

  // 清空指定会话现有的真实消息节点。
  function clearConversationMessages(conversation) {
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

  // 把一条持久消息追加到指定会话。
  function appendServerMessage(conversation, value) {
  if (conversation.messageIDs[value.id]) {
    return;
  }
  var message = messageContainer(value.author === "visitor" ? "visitor" : "assistant");
  message.setAttribute("data-message-id", value.id);
  var row = document.createElement("div");
  row.className = "cv-message-row";
  var bubble = document.createElement("div");
  bubble.className = "cv-message-bubble";
  bubble.textContent = value.body;
  row.appendChild(bubble);
  message.appendChild(row);
  message.appendChild(messageMeta(new Date(value.originatedAt)));
  conversation.messageIDs[value.id] = true;
  appendConversationNode(conversation, message);
  }

  // 生成客户端消息编号。
  function createClientMessageID() {
  if (window.crypto && typeof window.crypto.randomUUID === "function") {
    return window.crypto.randomUUID();
  }
  var bytes = new Uint8Array(16);
  window.crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  var hex = Array.from(bytes, function (value) {
    return value.toString(16).padStart(2, "0");
  }).join("");
  return hex.slice(0, 8) + "-" + hex.slice(8, 12) + "-" + hex.slice(12, 16) + "-" + hex.slice(16, 20) + "-" + hex.slice(20);
  }

  // 发送真实网站访客文本消息。
  function sendRealMessage(text) {
  if (!initialized || messageRequestPending || activeConversation.historyLoading) {
    return;
  }
  var conversation = activeConversation;
  if (conversation.pendingBody !== text || !conversation.pendingMessageID) {
    conversation.pendingBody = text;
    conversation.pendingMessageID = createClientMessageID();
  }
  messageRequestPending = true;
  $("cv-conversation-error").hidden = true;
  updateSendState();
  requestWebsiteJSON("/api/public/website-channels/" + encodeURIComponent(channelID) + "/messages", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
    clientMessageId: conversation.pendingMessageID,
    conversationId: conversation.id,
    body: text,
    }),
  })
    .then(function (result) {
    var wasDraft = conversation.id === null;
    conversation = upsertRealConversation(result.conversation, conversation);
    conversation.pendingMessageID = "";
    conversation.pendingBody = "";
    conversation.draft = "";
        appendServerMessage(conversation, result.message);
    conversation.historyLoaded = !wasDraft && conversation.historyLoaded;
    if (conversation === activeConversation) {
      if (input.value.trim() === text) {
      input.value = "";
      } else {
      conversation.draft = input.value;
      }
      intro.hidden = true;
          autosize();
    }
    renderRecentConversation();
    if (wasDraft) {
      loadConversationHistory(conversation);
    }
    })
    .catch(function (error) {
    if (conversation === activeConversation) {
      $("cv-conversation-error").textContent = error.message || requestFailedLabel;
      $("cv-conversation-error").hidden = false;
    }
    })
    .finally(function () {
    messageRequestPending = false;
    updateSendState();
    if (conversation === activeConversation) {
      input.focus();
    }
    });
  }

  function insertEmoji(emoji) {
    var start = input.selectionStart === null ? input.value.length : input.selectionStart;
    var end = input.selectionEnd === null ? input.value.length : input.selectionEnd;
    input.value = input.value.slice(0, start) + emoji + input.value.slice(end);
    var cursor = start + emoji.length;
    input.setSelectionRange(cursor, cursor);
    input.focus();
    autosize();
    updateSendState();
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

  function fileKind(file) {
    if (file.type.indexOf("image/") === 0) {
      return "image";
    }
    if (file.type.indexOf("video/") === 0) {
      return "video";
    }
    if (file.type.indexOf("audio/") === 0) {
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
  if (!previewMode) {
    return;
  }
    var files = listFiles(fileList);
    if (files.length === 0) {
      return;
    }
    appendVisitorMessage("", files);
    scheduleDemoReply();
    input.focus();
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
    var kind = fileKind(file);
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
      recordTime.textContent = formatDuration(Math.floor((Date.now() - recordingStartedAt) / 1000));
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
    var duration = Math.max(1, Math.floor((Date.now() - recordingStartedAt) / 1000));
    resetRecording();
    startConversation();
    var now = new Date();
    var message = messageContainer("visitor");
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
    var time = document.createElement("time");
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
      play.setAttribute("aria-label", playing ? playVoiceLabel : pauseVoiceLabel);
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
    message.appendChild(bubble);
    message.appendChild(messageMeta(now));
    messages.appendChild(message);
    updateConversationSummary(activeConversation, formatDuration(duration), now);
    scrollToBottom();
    scheduleDemoReply();
  }

  function relativeLuminance(hexColor) {
    var channels = hexColor.slice(1).match(/.{2}/g).map(function (channel) {
      var value = Number.parseInt(channel, 16) / 255;
      return value <= 0.04045 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4);
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
    var subtitle = typeof value.subtitle === "string" ? value.subtitle.trim() : "";
    var greeting = typeof value.greetingMessage === "string" ? value.greetingMessage.trim() : "";
    var themeColor = typeof value.themeColor === "string" ? value.themeColor.trim().toUpperCase() : "";
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
    forEachConversationNode('[data-greeting="true"] .cv-message-bubble', function (node) {
      node.textContent = greeting || defaultGreeting;
    });
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
  document.querySelectorAll("[data-new-conversation]").forEach(function (button) {
    button.addEventListener("click", beginNewConversation);
  });
  document.querySelectorAll("[data-resume-conversation]").forEach(function (button) {
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

  document.addEventListener("mousedown", rememberNativeContextMenuGesture, true);
  document.addEventListener("keydown", rememberKeyboardContextMenuGesture, true);
  document.addEventListener("contextmenu", allowOnlyNativeSecondaryButtonMenu, true);
  window.addEventListener("blur", function () {
    nativeContextMenuSource = "";
  });

  $("cv-help-input").addEventListener("input", filterHelp);
  input.addEventListener("input", function () {
  if (!previewMode && activeConversation.pendingBody !== input.value.trim()) {
    activeConversation.pendingBody = "";
    activeConversation.pendingMessageID = "";
  }
    autosize();
    updateSendState();
  });
  input.addEventListener("keydown", function (event) {
    if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
      return;
    }
    event.preventDefault();
    sendMessage();
  });
  input.addEventListener("paste", function (event) {
  if (!previewMode) {
    return;
  }
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
  if (!previewMode) {
    return;
  }
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
  $("cv-initialization-retry").addEventListener("click", initializeRealMessenger);
  $("cv-home-initialization-retry").addEventListener("click", initializeRealMessenger);
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
    if (event.key !== "Escape") {
      return;
    }
    if (lightbox && !lightbox.hidden) {
      lightbox.hidden = true;
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
  window.addEventListener("resize", autosize);
  if (!previewMode) {
    $("cv-attach").disabled = true;
    $("cv-voice").disabled = true;
    setNewConversationAvailability(false);
    initializeRealMessenger();
  }
})();
