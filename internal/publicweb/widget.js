// 网站渠道嵌入脚本：从 src 的 id 读取渠道，并在宿主页注入聊天角标。
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

  var style = document.createElement("style");
  style.textContent = [
    ":host{all:initial}",
    ".cv-panel{position:fixed;right:20px;bottom:88px;z-index:2147483000;width:380px;height:620px;max-width:calc(100vw - 24px);max-height:calc(100vh - 108px);overflow:hidden;border:1px solid rgba(15,23,42,.14);border-radius:16px;background:#fff;box-shadow:0 24px 80px rgba(15,23,42,.22);display:none}",
    '.cv-panel[data-open="true"]{display:block}',
    ".cv-frame{display:block;width:100%;height:100%;border:0;background:#fff}",
    ".cv-button{position:fixed;right:20px;bottom:24px;z-index:2147483001;width:52px;height:52px;border:0;border-radius:999px;background:#2563EB;color:#fff;box-shadow:0 16px 44px rgba(15,23,42,.25);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;padding:0}",
    ".cv-button:hover{transform:translateY(-1px)}",
    ".cv-button:focus-visible{outline:3px solid rgba(37,99,235,.35);outline-offset:3px}",
    ".cv-icon{display:block;width:22px;height:22px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}",
    ".cv-icon-close{display:none}",
    '.cv-button[data-open="true"] .cv-icon-chat{display:none}',
    '.cv-button[data-open="true"] .cv-icon-close{display:block}',
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

  function setOpen(next) {
    panel.dataset.open = next ? "true" : "false";
    button.dataset.open = next ? "true" : "false";
    panel.setAttribute("aria-hidden", next ? "false" : "true");
    button.setAttribute("aria-expanded", next ? "true" : "false");
    button.setAttribute("aria-label", next ? "Close chat" : "Open chat");
  }

  button.addEventListener("click", function () {
    setOpen(panel.dataset.open !== "true");
  });

  shadow.appendChild(style);
  shadow.appendChild(panel);
  shadow.appendChild(button);
  document.documentElement.appendChild(root);
})();
