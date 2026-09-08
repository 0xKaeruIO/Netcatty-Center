import type { Host, HostDraft } from "./types";

export function emptyHost(group = ""): HostDraft {
  return {
    id: null,
    label: "",
    hostname: "",
    port: 22,
    username: "",
    group,
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

export function cloneHostForEdit(host: Host): HostDraft {
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

export function hostStartupRules(host: Pick<HostDraft, "startupCommandRules">) {
  return Array.isArray(host.startupCommandRules) ? host.startupCommandRules : [];
}

export function hasUsableStartupRules(host: Pick<HostDraft, "startupCommandRules">) {
  return hostStartupRules(host).some((rule) => String(rule?.send ?? "").length > 0);
}

export function hostAuthLabel(host: Host) {
  const parts: string[] = [];
  if (host.password) parts.push("密码");
  if (host.privateKey) parts.push("密钥");
  if (host.startupCommandRunMode === "rules" && hasUsableStartupRules(host)) parts.push("启动规则");
  else if (host.startupCommand) parts.push("启动命令");
  return parts.join(" + ") || "未设置";
}

export function hostVisibilityLabel(host: Host, keys: { id: string; name: string }[]) {
  if (host.visibility === "keys") {
    const ids = Array.isArray(host.visibleKeyIds) ? host.visibleKeyIds : [];
    if (ids.length === 0) return "指定密钥（未选）";
    const names = ids.map((id) => keys.find((key) => key.id === id)?.name || id.slice(0, 8));
    return `指定 ${names.join("、")}`;
  }
  return "全部可见";
}

export function normalizeHostBody(host: HostDraft) {
  return {
    ...host,
    port: Number(host.port),
    tags: Array.isArray(host.tags) ? host.tags : String(host.tags || "").split(/[,，]/).map((t) => t.trim()).filter(Boolean),
  };
}
