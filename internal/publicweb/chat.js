// 访客聊天页本机演示：发送只渲染到当前页，不请求后端，刷新即清空。
(function () {
  var messages = document.getElementById("cv-messages");
  var composer = document.getElementById("cv-composer");
  var input = document.getElementById("cv-input");
  var emojiPanel = document.getElementById("cv-emoji");
  var fileInput = document.getElementById("cv-file-input");
  var imageInput = document.getElementById("cv-image-input");
  if (!messages || !composer || !input) {
    return;
  }

  var chrome = messages.closest(".cv-chrome");
  var lightbox = null;
  var avatarInitials = chrome.getAttribute("data-avatar") || "?";
  var demoReply = chrome.getAttribute("data-demo-reply") || "";
  var MAX_ATTACHMENT_COUNT = 10;
  var COMPOSER_MAX_HEIGHT = 160;
  var emojis = [
    "😀",
    "😄",
    "😁",
    "😆",
    "😅",
    "🤣",
    "😂",
    "🙂",
    "😉",
    "😊",
    "🥰",
    "😍",
    "🤩",
    "😘",
    "😋",
    "😜",
    "🤪",
    "🤔",
    "🤨",
    "😐",
    "😏",
    "😒",
    "🙄",
    "😬",
    "😌",
    "😔",
    "😴",
    "🥳",
    "😎",
    "😕",
    "😮",
    "😲",
    "😳",
    "🥺",
    "😢",
    "😭",
    "😱",
    "😖",
    "😞",
    "😩",
    "😫",
    "😤",
    "😡",
    "😠",
    "👻",
    "👍",
    "👎",
    "👏",
    "🙌",
    "🙏",
    "💪",
    "🤝",
    "✌️",
    "❤️",
    "💚",
    "💙",
    "💜",
    "🖤",
    "💔",
    "💯",
    "💕",
    "🔥",
    "⭐",
    "🌟",
    "✨",
    "🌈",
    "☀️",
    "🌙",
    "⚡",
    "🌸",
    "🌻",
    "🌹",
    "🍀",
    "🐶",
    "🐱",
    "🐰",
    "🐻",
    "🐼",
    "🐷",
    "🐸",
    "🦊",
    "🍎",
    "🍉",
    "🍓",
    "🍑",
    "🍔",
    "🍕",
    "🍣",
    "🍰",
    "🎂",
    "☕",
    "🍵",
    "🍺",
    "⚽",
    "🏀",
    "🎮",
    "🎯",
    "🏆",
    "🎉",
    "🎊",
    "🎁",
    "🚗",
    "✈️",
    "🚀",
    "🏠",
    "📱",
    "💻",
    "📷",
    "⏰",
    "💡",
    "💰",
    "🔑",
    "✅",
    "❌",
    "❓",
    "❗",
    "⚠️",
  ];

  function $(id) {
    return document.getElementById(id);
  }

  function autosize() {
    input.style.height = "auto";
    input.style.height =
      Math.min(input.scrollHeight, COMPOSER_MAX_HEIGHT) + "px";
    input.style.overflowY =
      input.scrollHeight > COMPOSER_MAX_HEIGHT ? "auto" : "hidden";
  }

  function fileKind(file) {
    var type = file.type || "";
    if (type.indexOf("image/") === 0) {
      return "image";
    }
    if (type.indexOf("video/") === 0) {
      return "video";
    }
    if (type.indexOf("audio/") === 0) {
      return "audio";
    }
    return "file";
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

  function formatTime(date) {
    return date.toLocaleTimeString(document.documentElement.lang, {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function scrollToBottom() {
    messages.scrollTop = messages.scrollHeight;
  }

  function listFiles(fileList) {
    var files = [];
    var limit = Math.min(fileList.length, MAX_ATTACHMENT_COUNT);
    for (var i = 0; i < limit; i += 1) {
      files.push(fileList[i]);
    }
    return files;
  }

  function addFiles(fileList) {
    var files = listFiles(fileList);
    if (files.length === 0) {
      return;
    }
    appendVisitorMessage("", files);
    scheduleDemoReply();
    input.focus();
  }

  function mediaNode(file) {
    var kind = fileKind(file);
    var url = URL.createObjectURL(file);
    if (kind === "image") {
      var imageButton = document.createElement("button");
      imageButton.type = "button";
      imageButton.className = "cv-card";
      var img = document.createElement("img");
      img.src = url;
      img.alt = file.name;
      imageButton.appendChild(img);
      imageButton.addEventListener("click", function () {
        openLightbox(url, file.name);
      });
      return imageButton;
    }
    if (kind === "video") {
      var videoWrap = document.createElement("div");
      videoWrap.className = "cv-card";
      var video = document.createElement("video");
      video.src = url;
      video.controls = true;
      video.playsInline = true;
      videoWrap.appendChild(video);
      return videoWrap;
    }
    if (kind === "audio") {
      var audioWrap = document.createElement("div");
      audioWrap.className = "cv-card cv-audio";
      var audio = document.createElement("audio");
      audio.src = url;
      audio.controls = true;
      audioWrap.appendChild(audio);
      return audioWrap;
    }
    var fileLink = document.createElement("a");
    fileLink.className = "cv-card cv-file";
    fileLink.href = url;
    fileLink.download = file.name;
    fileLink.target = "_blank";
    fileLink.rel = "noopener noreferrer";
    var name = document.createElement("div");
    name.className = "cv-file-name";
    name.textContent = file.name;
    var size = document.createElement("div");
    size.className = "cv-file-size";
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
    var img = document.createElement("img");
    img.src = url;
    img.alt = name || "";
    lightbox.appendChild(img);
    lightbox.hidden = false;
  }

  function visitorAvatar() {
    var avatar = document.createElement("div");
    avatar.className = "cv-avatar cv-avatar-visitor";
    avatar.setAttribute("aria-hidden", "true");
    avatar.innerHTML =
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>';
    return avatar;
  }

  function assistantAvatar() {
    var avatar = document.createElement("div");
    avatar.className = "cv-avatar cv-avatar-assistant";
    avatar.setAttribute("aria-hidden", "true");
    avatar.textContent = avatarInitials;
    return avatar;
  }

  function assetList(files) {
    var assets = document.createElement("div");
    assets.className = "cv-assets";
    files.forEach(function (file) {
      assets.appendChild(mediaNode(file));
    });
    return assets;
  }

  function appendVisitorMessage(text, files) {
    var hasText = Boolean(text);
    var hasFiles = files && files.length > 0;
    if (!hasText && !hasFiles) {
      return;
    }
    var msg = document.createElement("div");
    msg.className = "cv-msg cv-msg-visitor";
    var time = document.createElement("div");
    time.className = "cv-time";
    time.textContent = formatTime(new Date());
    var body = document.createElement("div");
    body.className = "cv-msg-body";
    var main = document.createElement("div");
    main.className = "cv-msg-main";
    if (hasText) {
      var bubble = document.createElement("div");
      bubble.className = "cv-bubble cv-bubble-visitor";
      var paragraph = document.createElement("p");
      paragraph.textContent = text;
      bubble.appendChild(paragraph);
      if (hasFiles) {
        bubble.appendChild(assetList(files));
      }
      main.appendChild(bubble);
    } else {
      main.appendChild(assetList(files));
    }
    body.appendChild(main);
    body.appendChild(visitorAvatar());
    msg.appendChild(time);
    msg.appendChild(body);
    messages.appendChild(msg);
    scrollToBottom();
  }

  function appendAssistantMessage(text) {
    if (!text) {
      return;
    }
    var msg = document.createElement("div");
    msg.className = "cv-msg cv-msg-assistant";
    var time = document.createElement("div");
    time.className = "cv-time";
    time.textContent = formatTime(new Date());
    var body = document.createElement("div");
    body.className = "cv-msg-body";
    var main = document.createElement("div");
    main.className = "cv-msg-main";
    var bubble = document.createElement("div");
    bubble.className = "cv-bubble cv-bubble-assistant";
    var paragraph = document.createElement("p");
    paragraph.textContent = text;
    bubble.appendChild(paragraph);
    main.appendChild(bubble);
    body.appendChild(assistantAvatar());
    body.appendChild(main);
    msg.appendChild(time);
    msg.appendChild(body);
    messages.appendChild(msg);
    scrollToBottom();
  }

  function scheduleDemoReply() {
    if (!demoReply) {
      return;
    }
    window.setTimeout(function () {
      appendAssistantMessage(demoReply);
    }, 400);
  }

  function sendMessage() {
    var text = input.value.replace(/^\s+|\s+$/g, "");
    if (!text) {
      return;
    }
    appendVisitorMessage(text, []);
    scheduleDemoReply();
    input.value = "";
    autosize();
    input.focus();
  }

  function insertEmoji(emoji) {
    var start = input.selectionStart || input.value.length;
    var end = input.selectionEnd || input.value.length;
    input.value = input.value.slice(0, start) + emoji + input.value.slice(end);
    var cursor = start + emoji.length;
    input.setSelectionRange(cursor, cursor);
    input.focus();
    autosize();
  }

  function pastedImageFiles(event) {
    var clipboard = event.clipboardData;
    if (!clipboard) {
      return [];
    }
    var files = [];
    var items = clipboard.items || [];
    for (var i = 0; i < items.length; i += 1) {
      var item = items[i];
      if (item.kind !== "file" || (item.type || "").indexOf("image/") !== 0) {
        continue;
      }
      var file = item.getAsFile();
      if (!file) {
        continue;
      }
      if (file.name) {
        files.push(file);
        continue;
      }
      files.push(
        new File(
          [file],
          "pasted-image-" + Date.now() + "-" + (files.length + 1) + ".png",
          { type: file.type || "image/png" },
        ),
      );
    }
    return files;
  }

  function fillEmojiPanel() {
    emojiPanel.innerHTML = "";
    emojis.forEach(function (emoji) {
      var button = document.createElement("button");
      button.type = "button";
      button.className = "cv-emoji-item";
      button.textContent = emoji;
      button.addEventListener("click", function () {
        insertEmoji(emoji);
        emojiPanel.hidden = true;
      });
      emojiPanel.appendChild(button);
    });
  }

  fillEmojiPanel();
  autosize();

  input.addEventListener("input", function () {
    autosize();
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

  var emojiToggle = $("cv-emoji-toggle");
  $("cv-attach").addEventListener("click", function () {
    fileInput.click();
  });
  fileInput.addEventListener("change", function () {
    addFiles(fileInput.files);
    fileInput.value = "";
  });
  $("cv-image").addEventListener("click", function () {
    imageInput.click();
  });
  imageInput.addEventListener("change", function () {
    addFiles(imageInput.files);
    imageInput.value = "";
  });
  emojiToggle.addEventListener("click", function () {
    emojiPanel.hidden = !emojiPanel.hidden;
  });
  document.addEventListener("click", function (event) {
    if (
      emojiPanel.hidden ||
      emojiPanel.contains(event.target) ||
      emojiToggle.contains(event.target)
    ) {
      return;
    }
    emojiPanel.hidden = true;
  });
})();
