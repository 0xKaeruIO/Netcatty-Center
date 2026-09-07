const state = {
  me: null,
  settings: null,
  needsSetup: false,
  view: "hosts",
  hosts: [],
  groups: [],
  keys: [],
  error: "",
  notice: "",
  query: "",
  expandedPaths: null,
  editing: null,
  issuing: null,
};

const app = document.getElementById("app");

async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `请求失败 (${response.status})`);
  }
  return data;
}

function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  attrs = disableAutofillAttrs(tag, attrs);
  for (const [key, value] of Object.entries(attrs)) {
    if (key === "class") el.className = value;
    else if (key.startsWith("on") && typeof value === "function") el.addEventListener(key.slice(2).toLowerCase(), value);
    else if (value === false || value == null) continue;
    else if (value === true) el.setAttribute(key, "");
    else el.setAttribute(key, String(value));
  }
  for (const child of children.flat()) {
    if (child == null || child === false) continue;
    el.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return el;
}

function disableAutofillAttrs(tag, attrs) {
  const next = { ...attrs };
  if (tag === "form") {
    if (next.autocomplete == null) next.autocomplete = "off";
    return next;
  }
  if (tag !== "input" && tag !== "textarea" && tag !== "select") return next;
  if (next.autocomplete == null) next.autocomplete = "off";
  if (next.autocorrect == null) next.autocorrect = "off";
  if (next.autocapitalize == null) next.autocapitalize = "off";
  if (next.spellcheck == null) next.spellcheck = "false";
  if (next["data-lpignore"] == null) next["data-lpignore"] = "true";
  if (next["data-1p-ignore"] == null) next["data-1p-ignore"] = true;
  if (next["data-bwignore"] == null) next["data-bwignore"] = "true";
  if (next["data-form-type"] == null) next["data-form-type"] = "other";
  const type = String(next.type || (tag === "textarea" ? "textarea" : "text")).toLowerCase();
  if (
    next.readonly == null
    && type !== "checkbox"
    && type !== "radio"
    && type !== "file"
    && type !== "hidden"
    && type !== "button"
    && type !== "submit"
  ) {
    next.readonly = true;
    const prevFocus = next.onFocus;
    next.onFocus = (event) => {
      event.target.removeAttribute("readonly");
      if (typeof prevFocus === "function") prevFocus(event);
    };
  }
  return next;
}

function formatTime(ts) {
  if (!ts) return "—";
  return new Date(ts).toLocaleString("zh-CN", { hour12: false });
}

async function boot() {
  try {
    const status = await api("/api/admin/setup-status");
    state.needsSetup = status.needsSetup;
    state.settings = status.settings;
    if (!status.needsSetup) {
      try {
        const me = await api("/api/admin/me");
        state.me = me.admin;
        state.settings = me.settings;
      } catch {
        state.me = null;
      }
    }
  } catch (err) {
    state.error = err.message;
  }
  render();
  if (state.me) await loadWorkspace();
}

async function loadWorkspace() {
  const [hosts, keys] = await Promise.all([
    api("/api/admin/hosts"),
    api("/api/admin/keys"),
  ]);
  state.hosts = hosts.hosts;
  state.groups = hosts.groups || [];
  state.keys = keys.keys;
  render();
}

function render() {
  app.replaceChildren();
  if (state.needsSetup) {
    app.append(renderAuth("setup"));
    return;
  }
  if (!state.me) {
    app.append(renderAuth("login"));
    return;
  }
  app.append(renderShell());
}

function renderAuth(mode) {
  const title = mode === "setup" ? "初始化管理员" : "管理员登录";
  const submitLabel = mode === "setup" ? "创建管理员并进入" : "登录";
  let username = "";
  let password = "";

  const form = h("form", {
    class: "auth-card",
    onSubmit: async (event) => {
      event.preventDefault();
      state.error = "";
      try {
        const path = mode === "setup" ? "/api/admin/setup" : "/api/admin/login";
        const result = await api(path, { method: "POST", body: { username, password } });
        state.needsSetup = false;
        state.me = result.admin;
        state.settings = result.settings;
        await loadWorkspace();
      } catch (err) {
        state.error = err.message;
        render();
      }
    },
  },
    h("h2", {}, title),
    h("p", { style: "margin:0 0 8px;color:var(--muted)" },
      mode === "setup"
        ? "首次启动需要创建一个管理员账号。之后用该账号维护主机目录，并签发客户端密钥。"
        : "使用组织中心管理员账号登录。"),
    state.error ? h("div", { class: "banner error" }, state.error) : null,
    field("用户名", h("input", {
      required: true,
      autocomplete: "off",
      onInput: (e) => { username = e.target.value; },
    })),
    field("密码", h("input", {
      type: "password",
      required: true,
      minlength: mode === "setup" ? "8" : undefined,
      autocomplete: "off",
      onInput: (e) => { password = e.target.value; },
    })),
    h("button", { class: "btn btn-primary", type: "submit" }, submitLabel),
  );

  return h("div", { class: "auth-screen" },
    h("section", { class: "auth-hero" },
      h("div", { class: "mark" },
        h("b", {}, "Netcatty Center"),
        h("span", {}, "运维服务器组织中心"),
      ),
      h("h1", {}, "主机目录由这里下发。"),
      h("p", {}, "管理员在此维护主机、登录密码和密钥。Netcatty 客户端用「地址 + 密钥」拉取目录后即可连接。"),
    ),
    h("section", { class: "auth-form" }, form),
  );
}

function renderShell() {
  return h("div", { class: "app-shell" },
    h("aside", { class: "rail" },
      h("div", { class: "mark" },
        h("b", {}, "NCC"),
        h("span", {}, state.settings?.centerName || "Netcatty Center"),
      ),
      h("nav", {},
        navBtn("hosts", "主机目录"),
        navBtn("keys", "客户端密钥"),
        navBtn("settings", "组织设置"),
      ),
      h("div", { class: "rail-foot" },
        h("div", {}, state.me.username),
        h("button", {
          class: "btn",
          onClick: async () => {
            await api("/api/admin/logout", { method: "POST" });
            state.me = null;
            render();
          },
        }, "退出"),
      ),
    ),
    h("main", { class: "main" },
      state.view === "hosts" ? renderHosts() : null,
      state.view === "keys" ? renderKeys() : null,
      state.view === "settings" ? renderSettings() : null,
    ),
  );
}

function navBtn(id, label) {
  return h("button", {
    class: state.view === id ? "active" : "",
    onClick: () => {
      state.view = id;
      state.error = "";
      state.notice = "";
      state.issuing = null;
      render();
    },
  }, label);
}

function renderHosts() {
  return h("section", {},
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "主机目录"),
        h("p", {}, `${state.hosts.length} 台主机。分组用 / 表示嵌套，客户端同步后会保留同样的目录结构。`),
      ),
      h("div", { class: "toolbar" },
        h("input", {
          class: "search",
          placeholder: "搜索主机 / IP / 分组",
          value: state.query,
          onInput: (e) => {
            state.query = e.target.value;
            refreshHostList();
          },
        }),
        h("button", {
          class: "btn",
          onClick: () => importHostsFromJSON(),
        }, "从 JSON 导入"),
        h("button", {
          class: "btn",
          onClick: () => exportHostsToJSON(),
        }, "导出为 JSON"),
        h("button", {
          class: "btn",
          onClick: () => createGroup(""),
        }, "新建分组"),
        h("button", {
          class: "btn btn-primary",
          onClick: () => {
            state.editing = emptyHost();
            render();
          },
        }, "添加主机"),
      ),
    ),
    state.error ? h("div", { class: "banner error" }, state.error) : null,
    state.notice ? h("div", { class: "banner ok" }, state.notice) : null,
    renderHostList(),
    state.editing ? renderHostDrawer() : null,
  );
}

function hostListMatches(host, q) {
  if (!q) return true;
  return [host.label, host.hostname, host.username, host.group, ...(host.tags || [])]
    .join(" ")
    .toLowerCase()
    .includes(q);
}

function renderHostList() {
  const q = state.query.trim().toLowerCase();
  const filtered = state.hosts.filter((host) => hostListMatches(host, q));
  const { tree, ungrouped } = buildGroupTree(filtered, q ? [] : state.groups);
  const allPaths = collectGroupPaths(tree);
  const searching = q !== "";
  const hasTree = tree.length > 0 || ungrouped.length > 0;

  if (!hasTree) {
    return h("div", { id: "host-list", class: "empty" },
      q ? "没有匹配的主机。" : "还没有主机或分组。添加第一台后，客户端即可按密钥拉取。",
    );
  }

  return h("div", { id: "host-list", class: "tree-wrap" },
    tree.length > 0
      ? h("div", { class: "tree-toolbar" },
        h("button", {
          class: "btn",
          disabled: searching,
          onClick: () => {
            state.expandedPaths = new Set(allPaths);
            refreshHostList();
          },
        }, "展开全部"),
        h("button", {
          class: "btn",
          disabled: searching,
          onClick: () => {
            state.expandedPaths = new Set();
            refreshHostList();
          },
        }, "折叠全部"),
      )
      : null,
    ...ungrouped.map((host) => renderHostRow(host, 0)),
    ...tree.flatMap((node) => renderGroupNode(node, 0, searching)),
  );
}

function refreshHostList() {
  const current = document.getElementById("host-list");
  if (!current) {
    render();
    return;
  }
  current.replaceWith(renderHostList());
}

function isExpanded(path, searching) {
  if (searching) return true;
  if (state.expandedPaths == null) return true;
  return state.expandedPaths.has(path);
}

function toggleExpanded(path) {
  const next = new Set(state.expandedPaths ?? collectGroupPaths(buildGroupTree(state.hosts, state.groups).tree));
  if (next.has(path)) next.delete(path);
  else next.add(path);
  state.expandedPaths = next;
  render();
}

function renderGroupNode(node, depth, searching) {
  const open = isExpanded(node.path, searching);
  const rows = [
    h("div", { class: "tree-row tree-group", style: `--depth:${depth}` },
      h("button", {
        class: "tree-toggle",
        type: "button",
        onClick: () => toggleExpanded(node.path),
      }, open ? "▾" : "▸"),
      h("div", { class: "tree-main", onClick: () => toggleExpanded(node.path) },
        h("strong", {}, node.name),
        h("span", { class: "tree-meta" }, `${node.totalHostCount} 台`, " · ", node.path),
      ),
      h("div", { class: "row-actions" },
        h("button", {
          class: "btn",
          onClick: () => createGroup(node.path),
        }, "子分组"),
        h("button", {
          class: "btn",
          onClick: () => {
            const host = emptyHost();
            host.group = node.path;
            state.editing = host;
            render();
          },
        }, "添加主机"),
        h("button", {
          class: "btn btn-danger",
          onClick: async () => {
            if (!confirm(`删除分组 ${node.path}？其中的主机会移到上一级。`)) return;
            const result = await api("/api/admin/groups/delete", { method: "POST", body: { path: node.path } });
            state.groups = result.groups || [];
            state.hosts = result.hosts || state.hosts;
            render();
          },
        }, "删除"),
        h("button", {
          class: "btn btn-danger",
          onClick: async () => {
            const count = node.totalHostCount || 0;
            if (!confirm(`删除分组 ${node.path} 及其下 ${count} 台主机？此操作无法撤销。`)) return;
            const result = await api("/api/admin/groups/delete", {
              method: "POST",
              body: { path: node.path, deleteHosts: true },
            });
            state.groups = result.groups || [];
            state.hosts = result.hosts || state.hosts;
            render();
          },
        }, "删除组及主机"),
      ),
    ),
  ];
  if (!open) return rows;
  for (const host of node.hosts) {
    rows.push(renderHostRow(host, depth + 1));
  }
  const children = Object.values(node.children).sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
  for (const child of children) {
    rows.push(...renderGroupNode(child, depth + 1, searching));
  }
  return rows;
}

function renderHostRow(host, depth) {
  return h("div", { class: "tree-row tree-host", style: `--depth:${depth}` },
    h("span", { class: "tree-toggle tree-toggle-spacer" }, ""),
    h("div", { class: "tree-main" },
      h("div", {}, host.label),
      h("div", { class: "tree-meta mono" },
        `${host.hostname}:${host.port}`,
        host.username ? ` · ${host.username}` : "",
        ` · ${host.protocol.toUpperCase()}`,
        ` · ${hostAuthLabel(host)}`,
        ` · ${hostVisibilityLabel(host)}`,
      ),
      h("div", {}, (host.tags || []).map((tag) => h("span", { class: "tag" }, tag))),
    ),
    h("div", { class: "row-actions" },
      h("button", { class: "btn", onClick: () => { state.editing = cloneHostForEdit(host); render(); } }, "编辑"),
      h("button", {
        class: "btn btn-danger",
        onClick: async () => {
          if (!confirm(`删除主机 ${host.label}？`)) return;
          await api(`/api/admin/hosts/${host.id}`, { method: "DELETE" });
          await loadWorkspace();
        },
      }, "删除"),
    ),
  );
}

function buildGroupTree(hosts, customGroups) {
  const root = {};
  const insertPath = (path, host) => {
    const parts = String(path || "").split("/").map((part) => part.trim()).filter(Boolean);
    let level = root;
    let current = "";
    parts.forEach((part, index) => {
      current = current ? `${current}/${part}` : part;
      if (!level[part]) {
        level[part] = { name: part, path: current, children: {}, hosts: [], totalHostCount: 0 };
      }
      if (host && index === parts.length - 1) {
        level[part].hosts.push(host);
      }
      level = level[part].children;
    });
  };
  for (const path of customGroups || []) {
    if (path) insertPath(path);
  }
  const ungrouped = [];
  for (const host of hosts) {
    if (host.group && String(host.group).trim()) insertPath(host.group, host);
    else ungrouped.push(host);
  }
  const countHosts = (node) => {
    let total = node.hosts.length;
    for (const child of Object.values(node.children)) {
      total += countHosts(child);
    }
    node.totalHostCount = total;
    return total;
  };
  const tree = Object.values(root).sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
  tree.forEach(countHosts);
  return { tree, ungrouped };
}

function collectGroupPaths(nodes) {
  const paths = [];
  const walk = (node) => {
    paths.push(node.path);
    for (const child of Object.values(node.children)) walk(child);
  };
  for (const node of nodes) walk(node);
  return paths;
}

async function exportHostsToJSON() {
  state.error = "";
  state.notice = "";
  try {
    const response = await fetch("/api/admin/hosts/export", { credentials: "same-origin" });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      throw new Error(data.error || `请求失败 (${response.status})`);
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "hosts-export.json";
    document.body.append(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    const hostCount = state.hosts.length;
    const groupCount = state.groups.length;
    state.notice = hostCount
      ? `已导出 ${hostCount} 台主机` + (groupCount ? `，分组 ${groupCount} 个` : "")
      : groupCount
        ? `已导出分组 ${groupCount} 个`
        : "已导出空目录";
    render();
  } catch (err) {
    state.error = err.message;
    render();
  }
}

async function importHostsFromJSON() {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".json,application/json";
  input.addEventListener("change", async () => {
    const file = input.files && input.files[0];
    if (!file) return;
    state.error = "";
    state.notice = "";
    try {
      const text = await file.text();
      let payload;
      try {
        payload = JSON.parse(text);
      } catch {
        throw new Error("JSON 文件无效");
      }
      const result = await api("/api/admin/hosts/import", { method: "POST", body: payload });
      const imported = result.imported ?? (result.hosts || []).length;
      const groupCount = (result.groups || []).length;
      state.notice = imported
        ? `已从 ${file.name} 导入 ${imported} 台主机` + (groupCount ? `，分组 ${groupCount} 个` : "")
        : `已导入分组，当前共 ${groupCount} 个`;
      await loadWorkspace();
    } catch (err) {
      state.error = err.message;
      render();
    }
  });
  input.click();
}

async function createGroup(parentPath) {
  const name = prompt(parentPath ? `在 ${parentPath} 下新建子分组` : "新建根分组", "");
  if (name == null) return;
  const trimmed = name.trim();
  if (!trimmed) return;
  if (/[\\/]/.test(trimmed)) {
    state.error = "分组名称不能包含 /";
    render();
    return;
  }
  const path = parentPath ? `${parentPath}/${trimmed}` : trimmed;
  try {
    const result = await api("/api/admin/groups", { method: "POST", body: { path } });
    state.groups = result.groups || [];
    if (state.expandedPaths) {
      for (const ancestor of path.split("/").map((_, i, parts) => parts.slice(0, i + 1).join("/"))) {
        state.expandedPaths.add(ancestor);
      }
    }
    render();
  } catch (err) {
    state.error = err.message;
    render();
  }
}

function emptyHost() {
  return {
    id: null,
    label: "",
    hostname: "",
    port: 22,
    username: "",
    group: "",
    tags: [],
    os: "linux",
    protocol: "ssh",
    deviceType: "general",
    notes: "",
    password: "",
    privateKey: "",
    passphrase: "",
    startupCommand: "",
    startupCommandRunMode: "paste",
    startupCommandRules: [],
    visibility: "all",
    visibleKeyIds: [],
  };
}

function cloneHostForEdit(host) {
  return {
    ...emptyHost(),
    ...host,
    startupCommandRules: Array.isArray(host.startupCommandRules)
      ? host.startupCommandRules.map((rule) => ({
        expect: rule?.expect ?? "",
        send: rule?.send ?? "",
      }))
      : [],
    visibleKeyIds: Array.isArray(host.visibleKeyIds) ? [...host.visibleKeyIds] : [],
  };
}

function hostStartupRules(host) {
  return Array.isArray(host.startupCommandRules) ? host.startupCommandRules : [];
}

function hasUsableStartupRules(host) {
  return hostStartupRules(host).some((rule) => String(rule?.send ?? "").length > 0);
}

function renderHostDrawer() {
  const host = state.editing;
  const isNew = !host.id;
  return h("div", { class: "overlay", onClick: (e) => {
    if (e.target.classList.contains("overlay")) {
      state.editing = null;
      render();
    }
  } },
    h("form", {
      class: host.visibility === "keys" ? "drawer drawer-wide" : "drawer",
      onSubmit: async (event) => {
        event.preventDefault();
        state.error = "";
        const body = {
          ...host,
          port: Number(host.port),
          tags: Array.isArray(host.tags) ? host.tags : String(host.tags || "").split(/[,，]/),
        };
        try {
          if (isNew) await api("/api/admin/hosts", { method: "POST", body });
          else await api(`/api/admin/hosts/${host.id}`, { method: "PUT", body });
          state.editing = null;
          await loadWorkspace();
        } catch (err) {
          state.error = err.message;
          render();
        }
      },
    },
      h("div", { class: host.visibility === "keys" ? "drawer-split" : "drawer-single" },
      h("div", { class: "drawer-form" },
      h("h2", {}, isNew ? "添加主机" : "编辑主机"),
      field("显示名称", bind(host, "label", { required: true })),
      h("div", { class: "grid-2" },
        field("主机名 / IP", bind(host, "hostname", { required: true, class: "mono" })),
        field("端口", bind(host, "port", { type: "number" })),
      ),
      h("div", { class: "grid-2" },
        field("用户名", bind(host, "username", { class: "mono" })),
        field("分组（用 / 嵌套，例如 production/web）", bind(host, "group", { placeholder: "production/web" })),
      ),
      field("标签（逗号分隔）", bind(host, "tags", {
        value: Array.isArray(host.tags) ? host.tags.join(", ") : host.tags,
        onInput: (e) => { host.tags = e.target.value; },
      })),
      h("div", { class: "grid-3" },
        field("协议", select(host, "protocol", [["ssh", "SSH"], ["telnet", "Telnet"]])),
        field("系统", select(host, "os", [["linux", "Linux"], ["windows", "Windows"], ["macos", "macOS"]])),
        field("设备类型", select(host, "deviceType", [["general", "通用服务器"], ["network", "网络设备"]])),
      ),
      field("备注", h("textarea", {
        value: host.notes,
        onInput: (e) => { host.notes = e.target.value; },
      })),
      field("登录密码", bind(host, "password", { type: "password" })),
      field("私钥（OpenSSH / PEM）", h("textarea", {
        class: "mono",
        value: host.privateKey || "",
        placeholder: "-----BEGIN OPENSSH PRIVATE KEY-----",
        onInput: (e) => { host.privateKey = e.target.value; },
      })),
      field("私钥口令（可选）", bind(host, "passphrase", { type: "password" })),
      field("连接后发送方式", h("select", {
        onChange: (e) => {
          host.startupCommandRunMode = e.target.value;
          if (host.startupCommandRunMode === "rules" && hostStartupRules(host).length === 0) {
            host.startupCommandRules = [{ expect: "", send: "" }];
          }
          render();
        },
      }, [
        ["paste", "一次性发送"],
        ["lineDelay", "逐行发送"],
        ["rules", "规则模式（Expect / Send）"],
      ].map(([value, label]) => h("option", {
        value,
        selected: (host.startupCommandRunMode || "paste") === value,
      }, label)))),
      host.startupCommandRunMode === "rules"
        ? renderStartupRules(host)
        : field("启动命令", h("textarea", {
          class: "mono",
          value: host.startupCommand || "",
          placeholder: "连接后执行的命令（例如：cd /app && ls）",
          onInput: (e) => { host.startupCommand = e.target.value; },
        })),
      h("p", { class: "hint" },
        host.startupCommandRunMode === "rules"
          ? "终端出现指定文本后发送对应命令。匹配文本留空则立即发送。可添加多条规则，按顺序跳转主机。规则会随目录下发给客户端。"
          : "SSH 连接建立后将自动执行该命令。规则模式与启动命令互斥。密码、密钥和启动规则都会随目录下发。"),
      field("目录可见范围", h("select", {
        onChange: (e) => {
          host.visibility = e.target.value;
          render();
        },
      }, [
        ["all", "全部可见"],
        ["keys", "仅特定密钥列表可见"],
      ].map(([value, label]) => h("option", {
        value,
        selected: (host.visibility || "all") === value,
      }, label)))),
      h("p", { class: "hint" },
        host.visibility === "keys"
          ? "只有勾选的客户端密钥拉取目录时能看到这台主机。未勾选任何密钥则对所有客户端隐藏。"
          : "所有有效客户端密钥都能拉取到这台主机。"),
      h("div", { class: "toolbar", style: "margin-top:8px" },
        h("button", { class: "btn btn-primary", type: "submit" }, "保存"),
        h("button", {
          class: "btn",
          type: "button",
          onClick: () => { state.editing = null; render(); },
        }, "取消"),
      ),
      ),
      host.visibility === "keys" ? renderVisibleKeyPicker(host) : null,
      ),
    ),
  );
}

function keyPermissionLabel(permission) {
  return permission === "readwrite" ? "可读可写" : "只读";
}

function renderKeys() {
  return h("section", {},
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "客户端密钥"),
        h("p", {}, "只读密钥可拉取机器列表并使用分享；可读可写密钥还能改机器列表。新密钥默认只读。"),
      ),
      h("button", {
        class: "btn btn-primary",
        onClick: () => {
          state.issuing = { name: "Netcatty 客户端", permission: "read" };
          state.error = "";
          render();
        },
      }, "签发密钥"),
    ),
    state.notice ? h("div", { class: "banner ok" }, state.notice) : null,
    state.error ? h("div", { class: "banner error" }, state.error) : null,
    state.keys.length === 0
      ? h("div", { class: "empty" }, "还没有客户端密钥。签发后即可用 curl 或 Netcatty 客户端拉取目录。")
      : h("div", { class: "table-wrap" },
        h("table", {},
          h("thead", {}, h("tr", {},
            h("th", {}, "名称"),
            h("th", {}, "权限"),
            h("th", {}, "密钥"),
            h("th", {}, "创建"),
            h("th", {}, "最近使用"),
            h("th", {}, "状态"),
            h("th", {}, ""),
          )),
          h("tbody", {}, state.keys.map((key) => h("tr", {},
            h("td", {}, key.name),
            h("td", {},
              h("span", { class: "tag" }, keyPermissionLabel(key.permission)),
            ),
            h("td", {}, key.plaintext
              ? h("div", { class: "row-actions" },
                h("code", { class: "mono secret-box", style: "padding:6px 8px;display:inline-block;max-width:420px" }, key.plaintext),
                h("button", {
                  class: "btn",
                  onClick: async () => {
                    await navigator.clipboard.writeText(key.plaintext);
                    state.notice = "已复制到剪贴板";
                    render();
                  },
                }, "复制"),
              )
              : h("span", { style: "color:var(--muted)" }, `${key.keyPrefix}…（旧密钥无法还原，请重新签发）`)),
            h("td", {}, formatTime(key.createdAt)),
            h("td", {}, formatTime(key.lastUsedAt)),
            h("td", {}, key.revokedAt ? "已吊销" : "有效"),
            h("td", {}, h("div", { class: "row-actions" },
              key.revokedAt
                ? [
                  h("button", {
                    class: "btn",
                    onClick: async () => {
                      await api(`/api/admin/keys/${key.id}/restore`, { method: "POST" });
                      state.notice = "已重新启用密钥";
                      await loadWorkspace();
                    },
                  }, "重新启用"),
                  h("button", {
                    class: "btn btn-danger",
                    onClick: async () => {
                      if (!confirm(`彻底删除密钥 ${key.name}？删除后无法恢复，使用该密钥的客户端将无法再拉取目录。`)) return;
                      await api(`/api/admin/keys/${key.id}/delete`, { method: "POST" });
                      state.notice = "已彻底删除密钥";
                      await loadWorkspace();
                    },
                  }, "彻底删除"),
                ]
                : [
                  h("button", {
                    class: "btn",
                    onClick: async () => {
                      const next = key.permission === "readwrite" ? "read" : "readwrite";
                      await api(`/api/admin/keys/${key.id}`, { method: "PUT", body: { permission: next } });
                      state.notice = `已改为${keyPermissionLabel(next)}`;
                      await loadWorkspace();
                    },
                  }, key.permission === "readwrite" ? "改为只读" : "改为可读可写"),
                  h("button", {
                    class: "btn btn-danger",
                    onClick: async () => {
                      if (!confirm("吊销后，使用该密钥的客户端将无法再拉取目录。之后仍可重新启用或彻底删除。")) return;
                      await api(`/api/admin/keys/${key.id}`, { method: "DELETE" });
                      await loadWorkspace();
                    },
                  }, "吊销"),
                ],
            )),
          ))),
        ),
      ),
    state.issuing ? renderIssueKeyDrawer() : null,
  );
}

function renderIssueKeyDrawer() {
  const draft = state.issuing;
  return h("div", { class: "overlay", onClick: (e) => {
    if (e.target.classList.contains("overlay")) {
      state.issuing = null;
      render();
    }
  } },
    h("form", {
      class: "drawer",
      onSubmit: async (event) => {
        event.preventDefault();
        state.error = "";
        try {
          await api("/api/admin/keys", {
            method: "POST",
            body: {
              name: draft.name,
              permission: draft.permission || "read",
            },
          });
          state.issuing = null;
          state.notice = `已签发${keyPermissionLabel(draft.permission || "read")}密钥`;
          await loadWorkspace();
        } catch (err) {
          state.error = err.message;
          render();
        }
      },
    },
      h("div", { class: "drawer-form" },
        h("h2", {}, "签发客户端密钥"),
        field("名称", bind(draft, "name", { required: true })),
        field("权限", h("select", {
          onChange: (e) => { draft.permission = e.target.value; },
        }, [
          ["read", "只读（默认）：可拉取机器列表，可开启/加入/关闭分享"],
          ["readwrite", "可读可写：还可新增、修改、删除机器列表"],
        ].map(([value, label]) => h("option", {
          value,
          selected: (draft.permission || "read") === value,
        }, label)))),
        h("p", { class: "hint" }, "只读不会阻止分享。分享的开启、加入和关闭对两种密钥都可用。"),
        h("div", { class: "toolbar", style: "margin-top:8px" },
          h("button", { class: "btn btn-primary", type: "submit" }, "签发"),
          h("button", {
            class: "btn",
            type: "button",
            onClick: () => { state.issuing = null; render(); },
          }, "取消"),
        ),
      ),
    ),
  );
}

function renderSettings() {
  let centerName = state.settings.centerName;
  const origin = location.origin;
  return h("section", {},
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "组织设置"),
        h("p", {}, "客户端稍后填写的地址就是当前站点 URL。"),
      ),
    ),
    h("form", {
      class: "panel",
      style: "max-width:560px;display:flex;flex-direction:column;gap:12px",
      onSubmit: async (event) => {
        event.preventDefault();
        const result = await api("/api/admin/settings", {
          method: "PUT",
          body: { centerName },
        });
        state.settings = result.settings;
        state.notice = "已保存";
        render();
      },
    },
      field("组织中心名称", h("input", {
        value: centerName,
        required: true,
        onInput: (e) => { centerName = e.target.value; },
      })),
      field("客户端地址", h("input", { class: "mono", value: origin, readonly: true })),
      h("p", { style: "color:var(--muted);margin:0" },
        "健康检查：",
        h("span", { class: "mono" }, `${origin}/api/v1/health`),
        h("br"),
        "目录接口：",
        h("span", { class: "mono" }, `${origin}/api/v1/catalog`),
      ),
      state.notice ? h("div", { class: "banner ok" }, state.notice) : null,
      h("button", { class: "btn btn-primary", type: "submit" }, "保存"),
    ),
    h("div", { class: "panel", style: "max-width:560px;margin-top:16px;display:flex;flex-direction:column;gap:8px" },
      h("h2", {}, "管理员账号"),
      h("p", { class: "hint" },
        "用户名和密码通过启动参数或环境变量指定，不会写入数据库。修改后重启进程即可生效。"),
      h("p", { class: "hint" },
        "启动参数：",
        h("span", { class: "mono" }, "--admin-user"),
        " / ",
        h("span", { class: "mono" }, "--admin-password")),
      h("p", { class: "hint" },
        "环境变量：",
        h("span", { class: "mono" }, "NCC_ADMIN_USER"),
        " / ",
        h("span", { class: "mono" }, "NCC_ADMIN_PASSWORD")),
      h("p", { class: "hint" }, "当前登录用户：", h("span", { class: "mono" }, state.me?.username || "—")),
    ),
  );
}

function renderStartupRules(host) {
  const rules = hostStartupRules(host);
  return h("div", { class: "rules" },
    h("span", { class: "field-label" }, "Expect / Send 规则"),
    ...rules.map((rule, index) => h("div", { class: "rule-card" },
      h("div", { class: "rule-head" },
        h("span", {}, `步骤 ${index + 1}`),
        h("button", {
          class: "btn",
          type: "button",
          onClick: () => {
            host.startupCommandRules = rules.filter((_, i) => i !== index);
            render();
          },
        }, "删除"),
      ),
      h("input", {
        class: "mono",
        value: rule.expect || "",
        placeholder: "匹配文本（留空则立即发送，例如 password:）",
        onInput: (e) => { rule.expect = e.target.value; },
      }),
      h("input", {
        class: "mono",
        value: rule.send || "",
        placeholder: "发送内容（例如 ssh user@jump）",
        onInput: (e) => { rule.send = e.target.value; },
      }),
    )),
    h("button", {
      class: "btn",
      type: "button",
      onClick: () => {
        host.startupCommandRules = [...rules, { expect: "", send: "" }];
        render();
      },
    }, "添加规则"),
  );
}

function renderVisibleKeyPicker(host) {
  const selected = new Set(Array.isArray(host.visibleKeyIds) ? host.visibleKeyIds : []);
  const keys = state.keys || [];
  const countText = () => `已选 ${(host.visibleKeyIds || []).length} / ${keys.length}`;
  let panel;

  const syncCount = () => {
    const el = panel?.querySelector("[data-key-count]");
    if (el) el.textContent = countText();
  };

  if (keys.length === 0) {
    return h("aside", { class: "drawer-keys" },
      h("h2", {}, "可见密钥"),
      h("p", { class: "hint" }, "还没有客户端密钥。请先到「客户端密钥」签发，再勾选可见范围。"),
    );
  }

  const applySelection = (ids) => {
    host.visibleKeyIds = ids;
    const set = new Set(ids);
    panel.querySelectorAll(".key-option").forEach((el) => {
      const box = el.querySelector("input[type=checkbox]");
      if (box) box.checked = set.has(el.getAttribute("data-key-id"));
    });
    syncCount();
  };

  panel = h("aside", { class: "drawer-keys" },
    h("div", { class: "key-list-head" },
      h("h2", {}, "可见密钥"),
      h("span", { class: "hint", "data-key-count": "" }, countText()),
    ),
    h("input", {
      class: "key-filter",
      placeholder: "搜索密钥名称 / 前缀",
      onInput: (e) => {
        const q = String(e.target.value || "").trim().toLowerCase();
        panel.querySelectorAll(".key-option").forEach((el) => {
          el.hidden = q !== "" && !String(el.getAttribute("data-search") || "").includes(q);
        });
      },
    }),
    h("div", { class: "toolbar" },
      h("button", {
        class: "btn",
        type: "button",
        onClick: () => applySelection(keys.filter((key) => !key.revokedAt).map((key) => key.id)),
      }, "全选有效"),
      h("button", {
        class: "btn",
        type: "button",
        onClick: () => applySelection([]),
      }, "清空"),
    ),
    h("div", { class: "key-list" },
      ...keys.map((key) => h("label", {
        class: "key-option",
        "data-key-id": key.id,
        "data-search": `${key.name} ${key.keyPrefix || ""} ${keyPermissionLabel(key.permission)} ${key.revokedAt ? "已吊销" : "有效"}`.toLowerCase(),
      },
        h("input", {
          type: "checkbox",
          checked: selected.has(key.id),
          onChange: (e) => {
            const next = new Set(host.visibleKeyIds || []);
            if (e.target.checked) next.add(key.id);
            else next.delete(key.id);
            host.visibleKeyIds = [...next];
            syncCount();
          },
        }),
        h("span", { class: "key-option-text" },
          h("strong", { class: "key-option-name" }, key.name, key.revokedAt ? "（已吊销）" : ""),
          h("span", { class: "key-option-meta mono" }, key.keyPrefix ? `${key.keyPrefix}…` : "—", " · ", keyPermissionLabel(key.permission), " · ", key.revokedAt ? "已吊销" : "有效"),
        ),
      )),
    ),
  );
  return panel;
}

function hostVisibilityLabel(host) {
  if (host.visibility === "keys") {
    const ids = Array.isArray(host.visibleKeyIds) ? host.visibleKeyIds : [];
    if (ids.length === 0) return "指定密钥（未选）";
    const names = ids.map((id) => state.keys.find((key) => key.id === id)?.name || id.slice(0, 8));
    return `指定 ${names.join("、")}`;
  }
  return "全部可见";
}

function hostAuthLabel(host) {
  const parts = [];
  if (host.password) parts.push("密码");
  if (host.privateKey) parts.push("密钥");
  if (host.startupCommandRunMode === "rules" && hasUsableStartupRules(host)) parts.push("启动规则");
  else if (host.startupCommand) parts.push("启动命令");
  return parts.join(" + ") || "未设置";
}

function field(label, control) {
  return h("label", { class: "field" }, h("span", {}, label), control);
}

function bind(host, key, extra = {}) {
  return h("input", {
    value: host[key] ?? "",
    onInput: (e) => { host[key] = e.target.value; },
    ...extra,
  });
}

function select(host, key, options) {
  return h("select", {
    onChange: (e) => { host[key] = e.target.value; },
  }, options.map(([value, label]) => h("option", {
    value,
    selected: host[key] === value,
  }, label)));
}

boot();
