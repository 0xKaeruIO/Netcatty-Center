const state = {
  me: null,
  settings: null,
  needsSetup: false,
  view: "hosts",
  hosts: [],
  keys: [],
  error: "",
  notice: "",
  query: "",
  editing: null,
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
      autocomplete: "username",
      onInput: (e) => { username = e.target.value; },
    })),
    field("密码", h("input", {
      type: "password",
      required: true,
      minlength: mode === "setup" ? "8" : undefined,
      autocomplete: mode === "setup" ? "new-password" : "current-password",
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
      render();
    },
  }, label);
}

function renderHosts() {
  const q = state.query.trim().toLowerCase();
  const rows = state.hosts.filter((host) => {
    if (!q) return true;
    return [host.label, host.hostname, host.username, host.group, ...(host.tags || [])]
      .join(" ")
      .toLowerCase()
      .includes(q);
  });

  return h("section", {},
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "主机目录"),
        h("p", {}, `${state.hosts.length} 台主机。客户端拉取时只包含连接元数据，不含密码或私钥。`),
      ),
      h("div", { class: "toolbar" },
        h("input", {
          class: "search",
          placeholder: "搜索主机 / IP / 分组",
          value: state.query,
          onInput: (e) => {
            state.query = e.target.value;
            render();
          },
        }),
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
    rows.length === 0
      ? h("div", { class: "empty" }, "还没有主机。添加第一台后，客户端即可按密钥拉取。")
      : h("div", { class: "table-wrap" },
        h("table", {},
          h("thead", {}, h("tr", {},
            h("th", {}, "名称"),
            h("th", {}, "地址"),
            h("th", {}, "用户"),
            h("th", {}, "登录"),
            h("th", {}, "分组"),
            h("th", {}, "标签"),
            h("th", {}, ""),
          )),
          h("tbody", {}, rows.map((host) => h("tr", {},
            h("td", {},
              h("div", {}, host.label),
              h("div", { class: "mono", style: "color:var(--muted);font-size:12px" }, host.protocol.toUpperCase()),
            ),
            h("td", { class: "mono" }, `${host.hostname}:${host.port}`),
            h("td", { class: "mono" }, host.username || "—"),
            h("td", {}, hostAuthLabel(host)),
            h("td", {}, host.group || "—"),
            h("td", {}, (host.tags || []).map((tag) => h("span", { class: "tag" }, tag))),
            h("td", {}, h("div", { class: "row-actions" },
              h("button", { class: "btn", onClick: () => { state.editing = { ...host }; render(); } }, "编辑"),
              h("button", {
                class: "btn btn-danger",
                onClick: async () => {
                  if (!confirm(`删除主机 ${host.label}？`)) return;
                  await api(`/api/admin/hosts/${host.id}`, { method: "DELETE" });
                  await loadWorkspace();
                },
              }, "删除"),
            )),
          ))),
        ),
      ),
    state.editing ? renderHostDrawer() : null,
  );
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
  };
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
      class: "drawer",
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
      h("h2", {}, isNew ? "添加主机" : "编辑主机"),
      field("显示名称", bind(host, "label", { required: true })),
      h("div", { class: "grid-2" },
        field("主机名 / IP", bind(host, "hostname", { required: true, class: "mono" })),
        field("端口", bind(host, "port", { type: "number" })),
      ),
      h("div", { class: "grid-2" },
        field("用户名", bind(host, "username", { class: "mono" })),
        field("分组", bind(host, "group", { placeholder: "production/web" })),
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
      field("登录密码", bind(host, "password", { type: "password", autocomplete: "off" })),
      field("私钥（OpenSSH / PEM）", h("textarea", {
        class: "mono",
        value: host.privateKey || "",
        placeholder: "-----BEGIN OPENSSH PRIVATE KEY-----",
        onInput: (e) => { host.privateKey = e.target.value; },
      })),
      field("私钥口令（可选）", bind(host, "passphrase", { type: "password", autocomplete: "off" })),
      h("p", { style: "color:var(--muted);margin:0;font-size:12px" }, "密码和密钥会随目录下发给已配置的 Netcatty 客户端，便于直接连接。"),
      h("div", { class: "toolbar", style: "margin-top:8px" },
        h("button", { class: "btn btn-primary", type: "submit" }, "保存"),
        h("button", {
          class: "btn",
          type: "button",
          onClick: () => { state.editing = null; render(); },
        }, "取消"),
      ),
    ),
  );
}

function renderKeys() {
  return h("section", {},
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "客户端密钥"),
        h("p", {}, "客户端配置「组织中心地址 + 密钥」。签发后可随时在此查看和复制。"),
      ),
      h("button", {
        class: "btn btn-primary",
        onClick: async () => {
          const name = prompt("密钥名称", "Netcatty 客户端") || "Netcatty 客户端";
          await api("/api/admin/keys", { method: "POST", body: { name } });
          state.notice = "已签发新密钥";
          await loadWorkspace();
        },
      }, "签发密钥"),
    ),
    state.notice ? h("div", { class: "banner ok" }, state.notice) : null,
    state.keys.length === 0
      ? h("div", { class: "empty" }, "还没有客户端密钥。签发后即可用 curl 或未来的 Netcatty 客户端拉取目录。")
      : h("div", { class: "table-wrap" },
        h("table", {},
          h("thead", {}, h("tr", {},
            h("th", {}, "名称"),
            h("th", {}, "密钥"),
            h("th", {}, "创建"),
            h("th", {}, "最近使用"),
            h("th", {}, "状态"),
            h("th", {}, ""),
          )),
          h("tbody", {}, state.keys.map((key) => h("tr", {},
            h("td", {}, key.name),
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
            h("td", {}, key.revokedAt ? null : h("button", {
              class: "btn btn-danger",
              onClick: async () => {
                if (!confirm("吊销后，使用该密钥的客户端将无法再拉取目录。")) return;
                await api(`/api/admin/keys/${key.id}`, { method: "DELETE" });
                await loadWorkspace();
              },
            }, "吊销")),
          ))),
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
  );
}

function hostAuthLabel(host) {
  const parts = [];
  if (host.password) parts.push("密码");
  if (host.privateKey) parts.push("密钥");
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
