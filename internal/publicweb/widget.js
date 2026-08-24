// 网站渠道嵌入脚本：在宿主页显示聊天挂件。
(function () {
  var script = document.currentScript;
  if (!script) {
    return;
  }

  var baseUrl;
  var channelId = "";
  var preview = false;
  var scriptUrl = new URL(script.src);
  baseUrl = scriptUrl.origin;
  channelId = (scriptUrl.searchParams.get("id") || "").trim();
  preview = scriptUrl.searchParams.get("preview") === "1";
  if (
    !preview &&
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      channelId,
    )
  ) {
    return;
  }

  var rootId = preview
    ? "cervi-widget-preview"
    : "cervi-widget-" + channelId.toLowerCase();
  if (document.getElementById(rootId)) {
    return;
  }

  var root = document.createElement("div");
  root.id = rootId;
  var shadow = root.attachShadow({ mode: "open" });
  var mobileQuery = window.matchMedia("(max-width: 640px)");
  var bottomInset = 0;
  var expanded = false;
  var frameReady = false;
  var previewConfig = null;
  var previewParentOrigin = "";
  var desktopPanelWidth = 400;
  var desktopPanelHeight = 640;
  var widgetCopies = {
    "zh-CN": {
      dialog: "Cervi 聊天",
      open: "打开聊天",
      close: "关闭聊天",
    },
    "en-US": {
      dialog: "Cervi chat",
      open: "Open chat",
      close: "Close chat",
    },
  };
  var widgetCopy = widgetCopies[preferredWidgetLocale()];
  var hostScrollLock = { applied: false, bodyOverflow: "", htmlOverflow: "" };

  var style = document.createElement("style");
  style.textContent = [
    ":host{all:initial}",
    "/*CV_THEME*/",
    ".cv-panel{box-sizing:border-box;position:fixed;z-index:2147483000;width:400px;height:640px;right:24px;bottom:96px;max-width:calc(100vw - 24px);max-height:calc(100dvh - 144px);overflow:hidden;border:1px solid rgba(24,24,27,.10);border-radius:24px;background:#fff;box-shadow:0 24px 72px rgba(15,23,42,.22);opacity:0;visibility:hidden;pointer-events:none;transform:translateY(8px) scale(.985);transform-origin:bottom right;transition:opacity .18s ease-out,transform .18s ease-out,visibility .18s}",
    '.cv-panel[data-open="true"]{opacity:1;visibility:visible;pointer-events:auto;transform:none}',
    ".cv-frame{display:block;width:100%;height:100%;border:0;background:#fff}",
    ".cv-button{box-sizing:border-box;position:fixed;right:24px;bottom:calc(24px + env(safe-area-inset-bottom,0px));z-index:2147483001;width:56px;height:56px;border:1px solid rgba(255,255,255,.16);border-radius:18px;background:var(--cv-theme);color:var(--cv-on-theme);box-shadow:var(--cv-launcher-shadow);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;padding:0;transition:transform .18s ease-out,box-shadow .18s ease-out,border-radius .18s ease-out}",
    ".cv-button:hover{transform:translateY(-2px)}",
    '.cv-button[data-open="true"]{border-radius:999px}',
    ".cv-button:focus-visible{outline:3px solid var(--cv-focus);outline-offset:3px}",
    ".cv-icon{display:block;width:22px;height:22px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}",
    ".cv-icon-close{display:none}",
    '.cv-button[data-open="true"] .cv-icon-chat{display:none}',
    '.cv-button[data-open="true"] .cv-icon-close{display:block}',
    ".cv-badge{position:absolute;top:-3px;right:-3px;width:12px;height:12px;border:2px solid #fff;border-radius:999px;background:#dc2626;box-shadow:0 2px 8px rgba(15,23,42,.2)}",
    ".cv-badge[hidden]{display:none}",
    "@media (prefers-reduced-motion:reduce){.cv-button,.cv-panel{transition:none}.cv-button:hover,.cv-button:active{transform:none}.cv-panel{transform:none}}",
  ].join("");

  var panel = document.createElement("div");
  panel.className = "cv-panel";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", widgetCopy.dialog);
  panel.setAttribute("aria-hidden", "true");

  var frame = document.createElement("iframe");
  frame.className = "cv-frame";
  frame.title = widgetCopy.dialog;
  frame.loading = preview ? "eager" : "lazy";
  frame.referrerPolicy = "strict-origin-when-cross-origin";
  frame.allow = "clipboard-write";
  frame.src = preview
    ? baseUrl + "/embed/preview/frame"
    : baseUrl + "/embed/widget/" + encodeURIComponent(channelId);
  panel.appendChild(frame);

  var button = document.createElement("button");
  button.className = "cv-button";
  button.type = "button";
  button.setAttribute("aria-expanded", "false");
  button.setAttribute("aria-label", widgetCopy.open);
  button.appendChild(
    createIcon("cv-icon-chat", [
      [
        "path",
        {
          d: "M21 12a8.6 8.6 0 0 1-9 8.5 9.8 9.8 0 0 1-3.8-.8L3 21l1.4-4.7A8.2 8.2 0 0 1 3 12a8.6 8.6 0 0 1 9-8.5A8.6 8.6 0 0 1 21 12Z",
        },
      ],
      ["path", { d: "M8.5 11.5h7" }],
      ["path", { d: "M8.5 14.5H13" }],
    ]),
  );
  var unreadBadge = document.createElement("span");
  unreadBadge.className = "cv-badge";
  unreadBadge.hidden = true;
  unreadBadge.setAttribute("aria-hidden", "true");
  button.appendChild(unreadBadge);
  button.appendChild(
    createIcon("cv-icon-close", [
      ["path", { d: "M18 6 6 18" }],
      ["path", { d: "m6 6 12 12" }],
    ]),
  );

  function createIcon(className, nodes) {
    var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("class", "cv-icon " + className);
    svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
    nodes.forEach(function (node) {
      var el = document.createElementNS("http://www.w3.org/2000/svg", node[0]);
      Object.keys(node[1]).forEach(function (key) {
        el.setAttribute(key, node[1][key]);
      });
      svg.appendChild(el);
    });
    return svg;
  }

  // 管理端预览固定使用桌面布局，公开挂件按宿主页视口响应。
  function isMobile() {
    return !preview && mobileQuery.matches;
  }

  function preferredWidgetLocale() {
    var first = window.navigator.languages[0] || window.navigator.language;
    return /^zh(?:-|$)/i.test(first.trim()) ? "zh-CN" : "en-US";
  }

  function fittedDesktopPanelSize(maxScale, topGap, bottomGap) {
    var availableWidth = Math.max(0, window.innerWidth - 48);
    var availableHeight = Math.max(
      0,
      window.innerHeight - topGap - bottomGap,
    );
    var scale = Math.max(
      0,
      Math.min(
        maxScale,
        availableWidth / desktopPanelWidth,
        availableHeight / desktopPanelHeight,
      ),
    );
    return {
      width: desktopPanelWidth * scale + "px",
      height: desktopPanelHeight * scale + "px",
    };
  }

  function relativeLuminance(hexColor) {
    var channels = hexColor.slice(1).match(/.{2}/g).map(function (channel) {
      var value = Number.parseInt(channel, 16) / 255;
      return value <= 0.04045
        ? value / 12.92
        : Math.pow((value + 0.055) / 1.055, 2.4);
    });
    return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  }

  function applyPreviewTheme(value) {
    var color =
      typeof value.themeColor === "string"
        ? value.themeColor.trim().toUpperCase()
        : "";
    if (!/^#[0-9A-F]{6}$/.test(color)) {
      return;
    }
    var luminance = relativeLuminance(color);
    var whiteContrast = 1.05 / (luminance + 0.05);
    var darkContrast =
      (luminance + 0.05) / (relativeLuminance("#1C1917") + 0.05);
    var focus = "color-mix(in srgb, " + color + " 40%, transparent)";
    var shadow =
      "0 10px 28px color-mix(in srgb, " + color + " 42%, transparent)";
    if (whiteContrast < 3) {
      focus = "rgba(28, 25, 23, 0.35)";
      shadow = "0 8px 24px rgba(28, 25, 23, 0.12)";
    }
    root.style.setProperty("--cv-theme", color);
    root.style.setProperty(
      "--cv-on-theme",
      whiteContrast >= darkContrast ? "#FFFFFF" : "#1C1917",
    );
    root.style.setProperty("--cv-focus", focus);
    root.style.setProperty("--cv-launcher-shadow", shadow);
  }

  function sendPreviewConfig() {
    if (!preview || !previewConfig || !frame.contentWindow) {
      return;
    }
    frame.contentWindow.postMessage(previewConfig, baseUrl);
  }

  function notifyPreviewReady() {
    if (!preview || window.parent === window) {
      return;
    }
    window.parent.postMessage(
      { type: "cervi:preview-ready" },
      previewParentOrigin || "*",
    );
  }

  function syncHostScrollLock() {
    var shouldLock = panel.dataset.open === "true" && isMobile();
    if (shouldLock === hostScrollLock.applied) {
      return;
    }
    if (shouldLock) {
      hostScrollLock.bodyOverflow = document.body.style.overflow;
      hostScrollLock.htmlOverflow = document.documentElement.style.overflow;
      document.body.style.overflow = "hidden";
      document.documentElement.style.overflow = "hidden";
      hostScrollLock.applied = true;
      return;
    }
    document.body.style.overflow = hostScrollLock.bodyOverflow;
    document.documentElement.style.overflow = hostScrollLock.htmlOverflow;
    hostScrollLock.applied = false;
  }

  function applyLayout() {
    var open = panel.dataset.open === "true";
    if (isMobile()) {
      expanded = false;
      panel.style.left = "0";
      panel.style.right = "0";
      panel.style.top = "0";
      panel.style.bottom = "0";
      panel.style.width = "100vw";
      panel.style.height = "100dvh";
      panel.style.maxWidth = "none";
      panel.style.maxHeight = "none";
      panel.style.borderRadius = "0";
      panel.style.border = "0";
      panel.style.boxShadow = "none";
      frame.style.height =
        bottomInset > 0 ? "calc(100% - " + bottomInset + "px)" : "";
      button.style.display = open && frameReady ? "none" : "inline-flex";
    } else if (expanded) {
      var expandedSize = fittedDesktopPanelSize(1.15, 24, 24);
      panel.style.left = "";
      panel.style.right = "24px";
      panel.style.top = "";
      panel.style.bottom = "24px";
      panel.style.width = expandedSize.width;
      panel.style.height = expandedSize.height;
      panel.style.maxWidth = "none";
      panel.style.maxHeight = "none";
      panel.style.borderRadius = "24px";
      panel.style.border = "1px solid rgba(24,24,27,.10)";
      panel.style.boxShadow = "0 28px 90px rgba(15,23,42,.24)";
      frame.style.height = "";
      button.style.display = open ? "none" : "inline-flex";
    } else {
      var defaultSize = fittedDesktopPanelSize(1, 48, 96);
      panel.style.left = "";
      panel.style.right = "";
      panel.style.top = "";
      panel.style.bottom = "";
      panel.style.width = defaultSize.width;
      panel.style.height = defaultSize.height;
      panel.style.maxWidth = "none";
      panel.style.maxHeight = "none";
      panel.style.borderRadius = "";
      panel.style.border = "";
      panel.style.boxShadow = "";
      frame.style.height = "";
      button.style.display = "inline-flex";
    }
    syncHostScrollLock();
  }

  function setOpen(next) {
    if (!next) {
      expanded = false;
    }
    panel.dataset.open = next ? "true" : "false";
    button.dataset.open = next ? "true" : "false";
    panel.setAttribute("aria-hidden", next ? "false" : "true");
    button.setAttribute("aria-expanded", next ? "true" : "false");
    button.setAttribute("aria-label", next ? widgetCopy.close : widgetCopy.open);
    applyLayout();
    syncFrameState();
    if (next) {
      frame.focus();
    } else {
      button.focus();
    }
  }

  function setBottomInset(px) {
    var next = Math.max(0, Math.min(2000, Math.round(Number(px) || 0)));
    bottomInset = next;
    applyLayout();
  }

  function handleViewportModeChange() {
    applyLayout();
    syncFrameState();
  }

  function syncFrameState() {
    if (!frame.contentWindow) {
      return;
    }
    frame.contentWindow.postMessage(
      {
        type: "cervi:widget-state",
        visible: panel.dataset.open === "true",
        expanded: expanded,
        expandable: !isMobile(),
      },
      baseUrl,
    );
  }

  button.addEventListener("click", function () {
    setOpen(panel.dataset.open !== "true");
  });

  window.addEventListener("message", function (event) {
    if (
      preview &&
      event.source === window.parent &&
      event.data &&
      event.data.type === "cervi:preview-config"
    ) {
      previewParentOrigin = event.origin === "null" ? "" : event.origin;
      previewConfig = event.data;
      applyPreviewTheme(event.data.value || {});
      sendPreviewConfig();
      return;
    }
    if (event.origin !== baseUrl || event.source !== frame.contentWindow) {
      return;
    }
    if (!event.data || typeof event.data.type !== "string") {
      return;
    }
    if (event.data.type === "cervi:frame-ready") {
      frameReady = true;
      applyLayout();
      return;
    }
    if (event.data.type === "cervi:close") {
      setOpen(false);
      return;
    }
    if (event.data.type === "cervi:unread") {
      unreadBadge.hidden = event.data.unread !== true;
      return;
    }
    if (event.data.type === "cervi:preview-ready") {
      notifyPreviewReady();
      return;
    }
    if (event.data.type === "cervi:toggle-expand" && !isMobile()) {
      expanded = !expanded;
      applyLayout();
      syncFrameState();
    }
  });

  document.addEventListener("click", function (event) {
    var trigger = event.target.closest("[data-cervi-open]");
    if (!trigger) {
      return;
    }
    var targetId = trigger.getAttribute("data-cervi-open").trim();
    if (targetId !== "" && targetId.toLowerCase() !== channelId.toLowerCase()) {
      return;
    }
    event.preventDefault();
    setOpen(true);
  });

  mobileQuery.addEventListener("change", handleViewportModeChange);
  frame.addEventListener("load", function () {
    syncFrameState();
    sendPreviewConfig();
  });
  window.addEventListener("resize", applyLayout);

  var api = {
    show: function () {
      setOpen(true);
    },
    hide: function () {
      setOpen(false);
    },
    setBottomInset: setBottomInset,
  };
  window.Cervi = api;

  shadow.appendChild(style);
  shadow.appendChild(panel);
  shadow.appendChild(button);
  document.documentElement.appendChild(root);
  applyLayout();
})();
