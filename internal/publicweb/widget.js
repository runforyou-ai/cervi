// 网站渠道嵌入脚本：在宿主页显示聊天挂件。
(function () {
  var script = document.currentScript;
  if (!script) {
    return;
  }

  var baseUrl;
  var channelId = "";
  try {
    var scriptUrl = new URL(script.getAttribute("src") || "", document.baseURI);
    baseUrl = scriptUrl.origin;
    channelId = (scriptUrl.searchParams.get("id") || "").trim();
  } catch (error) {
    return;
  }
  if (
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      channelId,
    )
  ) {
    return;
  }

  var rootId = "cervi-widget-" + channelId.toLowerCase();
  if (document.getElementById(rootId)) {
    return;
  }

  var root = document.createElement("div");
  root.id = rootId;
  var shadow = root.attachShadow({ mode: "open" });
  var mobileQuery = window.matchMedia(
    "(max-width: 640px), (pointer: coarse)",
  );
  var bottomInset = 0;
  var hostScrollLock = { applied: false, bodyOverflow: "", htmlOverflow: "" };

  var style = document.createElement("style");
  style.textContent = [
    ":host{all:initial}",
    "/*CV_THEME*/",
    ".cv-panel{position:fixed;z-index:2147483000;width:380px;height:620px;right:20px;bottom:88px;max-width:calc(100vw - 16px);max-height:calc(100vh - 108px);overflow:hidden;border:1px solid rgba(15,23,42,.14);border-radius:16px;background:#fff;box-shadow:0 24px 80px rgba(15,23,42,.22);opacity:0;visibility:hidden;pointer-events:none;transform:translateY(8px);transition:opacity .18s ease,transform .18s ease,visibility .18s}",
    '.cv-panel[data-open="true"]{opacity:1;visibility:visible;pointer-events:auto;transform:none}',
    ".cv-frame{display:block;width:100%;height:100%;border:0;background:#fff}",
    ".cv-button{position:fixed;right:20px;bottom:24px;z-index:2147483001;width:52px;height:52px;border:0;border-radius:999px;background:var(--cv-theme);color:var(--cv-on-theme);box-shadow:var(--cv-launcher-shadow);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;padding:0;transition:transform .18s ease,box-shadow .18s ease}",
    ".cv-button:hover{transform:translateY(-1px)}",
    ".cv-button:focus-visible{outline:3px solid var(--cv-focus);outline-offset:3px}",
    ".cv-icon{display:block;width:22px;height:22px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}",
    ".cv-icon-close{display:none}",
    '.cv-button[data-open="true"] .cv-icon-chat{display:none}',
    '.cv-button[data-open="true"] .cv-icon-close{display:block}',
    "@media (prefers-reduced-motion:reduce){.cv-button,.cv-panel{transition:none}.cv-button:hover,.cv-button:active{transform:none}.cv-panel{transform:none}}",
  ].join("");

  var panel = document.createElement("div");
  panel.className = "cv-panel";
  panel.setAttribute("aria-hidden", "true");

  var frame = document.createElement("iframe");
  frame.className = "cv-frame";
  frame.title = "Cervi chat";
  frame.loading = "lazy";
  frame.referrerPolicy = "strict-origin-when-cross-origin";
  frame.allow = "clipboard-write";
  frame.src = baseUrl + "/embed/widget/" + encodeURIComponent(channelId);
  panel.appendChild(frame);

  var button = document.createElement("button");
  button.className = "cv-button";
  button.type = "button";
  button.setAttribute("aria-expanded", "false");
  button.setAttribute("aria-label", "Open chat");
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

  function isMobile() {
    return mobileQuery.matches;
  }

  function syncHostScrollLock() {
    var shouldLock = panel.dataset.open === "true" && isMobile();
    if (shouldLock === hostScrollLock.applied || !document.body) {
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
      button.style.display = open ? "none" : "inline-flex";
    } else {
      panel.style.left = "";
      panel.style.right = "";
      panel.style.top = "";
      panel.style.bottom = "";
      panel.style.width = "";
      panel.style.height = "";
      panel.style.maxWidth = "";
      panel.style.maxHeight = "";
      panel.style.borderRadius = "";
      panel.style.border = "";
      panel.style.boxShadow = "";
      frame.style.height = "";
      button.style.display = "inline-flex";
    }
    syncHostScrollLock();
  }

  function setOpen(next) {
    panel.dataset.open = next ? "true" : "false";
    button.dataset.open = next ? "true" : "false";
    panel.setAttribute("aria-hidden", next ? "false" : "true");
    button.setAttribute("aria-expanded", next ? "true" : "false");
    button.setAttribute("aria-label", next ? "Close chat" : "Open chat");
    applyLayout();
  }

  function setBottomInset(px) {
    var next = Math.max(0, Math.min(2000, Math.round(Number(px) || 0)));
    bottomInset = next;
    applyLayout();
  }

  button.addEventListener("click", function () {
    setOpen(panel.dataset.open !== "true");
  });

  window.addEventListener("message", function (event) {
    if (event.origin !== baseUrl || event.source !== frame.contentWindow) {
      return;
    }
    if (!event.data || typeof event.data.type !== "string") {
      return;
    }
    if (event.data.type === "cervi:close") {
      setOpen(false);
    }
  });

  document.addEventListener("click", function (event) {
    var trigger = event.target.closest("[data-cervi-open]");
    if (!trigger) {
      return;
    }
    var targetId = (trigger.getAttribute("data-cervi-open") || "").trim();
    if (targetId !== "" && targetId.toLowerCase() !== channelId.toLowerCase()) {
      return;
    }
    event.preventDefault();
    setOpen(true);
  });

  mobileQuery.addEventListener("change", applyLayout);
  window.addEventListener("resize", applyLayout);
  window.addEventListener("orientationchange", applyLayout);

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
