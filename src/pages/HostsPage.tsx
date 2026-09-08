import { App, Button, Input, Space, Tag } from "antd";
import { type ReactNode, useMemo, useState } from "react";
import { api } from "../api";
import { HostDrawer } from "../components/HostDrawer";
import { cloneHostForEdit, emptyHost, hostAuthLabel, hostVisibilityLabel, normalizeHostBody } from "../hosts";
import { noAuto } from "../theme";
import { ancestorPaths, buildGroupTree, collectGroupPaths, hostListMatches } from "../tree";
import type { APIKey, GroupNode, Host, HostDraft } from "../types";

type Props = {
  hosts: Host[];
  groups: string[];
  keys: APIKey[];
  onReload: () => Promise<void>;
  onGroups: (groups: string[], hosts?: Host[]) => void;
};

export function HostsPage({ hosts, groups, keys, onReload, onGroups }: Props) {
  const { message, modal } = App.useApp();
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Set<string> | null>(null);
  const [editing, setEditing] = useState<HostDraft | null>(null);
  const q = query.trim().toLowerCase();
  const searching = q !== "";

  const filtered = useMemo(
    () => hosts.filter((host) => hostListMatches(host, q)),
    [hosts, q],
  );
  const { tree, ungrouped } = useMemo(
    () => buildGroupTree(filtered, q ? [] : groups),
    [filtered, groups, q],
  );
  const allPaths = useMemo(() => collectGroupPaths(tree), [tree]);
  const hasTree = tree.length > 0 || ungrouped.length > 0;

  const isOpen = (path: string) => {
    if (searching) return true;
    if (expanded == null) return true;
    return expanded.has(path);
  };

  const toggle = (path: string) => {
    const next = new Set(expanded ?? collectGroupPaths(buildGroupTree(hosts, groups).tree));
    if (next.has(path)) next.delete(path);
    else next.add(path);
    setExpanded(next);
  };

  const createGroup = (parentPath: string) => {
    let name = "";
    modal.confirm({
      title: parentPath ? `在 ${parentPath} 下新建子分组` : "新建根分组",
      content: (
        <Input
          {...noAuto}
          autoFocus
          placeholder="分组名称，不要包含 /"
          onChange={(e) => {
            name = e.target.value;
          }}
        />
      ),
      okText: "创建",
      cancelText: "取消",
      onOk: async () => {
        const trimmed = name.trim();
        if (!trimmed) return Promise.reject();
        if (/[\\/]/.test(trimmed)) {
          message.error("分组名称不能包含 /");
          return Promise.reject();
        }
        const path = parentPath ? `${parentPath}/${trimmed}` : trimmed;
        const result = await api<{ groups: string[] }>("/api/admin/groups", { method: "POST", body: { path } });
        onGroups(result.groups || []);
        setExpanded((prev) => {
          const next = new Set(prev ?? allPaths);
          for (const ancestor of ancestorPaths(path)) next.add(ancestor);
          return next;
        });
      },
    });
  };

  const importHosts = () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".json,application/json";
    input.addEventListener("change", async () => {
      const file = input.files?.[0];
      if (!file) return;
      try {
        const text = await file.text();
        let payload: unknown;
        try {
          payload = JSON.parse(text);
        } catch {
          throw new Error("JSON 文件无效");
        }
        const result = await api<{ imported?: number; hosts?: Host[]; groups?: string[] }>(
          "/api/admin/hosts/import",
          { method: "POST", body: payload },
        );
        const imported = result.imported ?? (result.hosts || []).length;
        const groupCount = (result.groups || []).length;
        message.success(
          imported
            ? `已从 ${file.name} 导入 ${imported} 台主机` + (groupCount ? `，分组 ${groupCount} 个` : "")
            : `已导入分组，当前共 ${groupCount} 个`,
        );
        await onReload();
      } catch (err) {
        message.error(err instanceof Error ? err.message : "导入失败");
      }
    });
    input.click();
  };

  const exportHosts = async () => {
    try {
      const response = await fetch("/api/admin/hosts/export", { credentials: "same-origin" });
      if (!response.ok) {
        const data = (await response.json().catch(() => ({}))) as { error?: string };
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
      const hostCount = hosts.length;
      const groupCount = groups.length;
      message.success(
        hostCount
          ? `已导出 ${hostCount} 台主机` + (groupCount ? `，分组 ${groupCount} 个` : "")
          : groupCount
            ? `已导出分组 ${groupCount} 个`
            : "已导出空目录",
      );
    } catch (err) {
      message.error(err instanceof Error ? err.message : "导出失败");
    }
  };

  return (
    <section className="ncc-page">
      <div className="ncc-page-head">
        <div>
          <h1>主机目录</h1>
          <p>{hosts.length} 台主机。分组用 / 表示嵌套，客户端同步后会保留同样的目录结构。</p>
        </div>
        <div className="ncc-toolbar">
          <Input
            {...noAuto}
            allowClear
            placeholder="搜索主机 / IP / 分组"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            style={{ width: 220 }}
          />
          <Button onClick={importHosts}>从 JSON 导入</Button>
          <Button onClick={exportHosts}>导出为 JSON</Button>
          <Button onClick={() => createGroup("")}>新建分组</Button>
          <Button type="primary" onClick={() => setEditing(emptyHost())}>添加主机</Button>
        </div>
      </div>

      {!hasTree ? (
        <div className="ncc-empty">
          {q ? "没有匹配的主机。" : "还没有主机或分组。添加第一台后，客户端即可按密钥拉取。"}
        </div>
      ) : (
        <div className="ncc-tree">
          {tree.length > 0 ? (
            <div className="ncc-tree-toolbar">
              <Button disabled={searching} onClick={() => setExpanded(new Set(allPaths))}>展开全部</Button>
              <Button disabled={searching} onClick={() => setExpanded(new Set())}>折叠全部</Button>
            </div>
          ) : null}
          {ungrouped.map((host) => (
            <HostRow key={host.id} host={host} depth={0} keys={keys} onEdit={() => setEditing(cloneHostForEdit(host))} onReload={onReload} />
          ))}
          {tree.flatMap((node) =>
            renderGroup(node, 0, isOpen, toggle, createGroup, setEditing, keys, onReload, onGroups, modal),
          )}
        </div>
      )}

      {editing ? (
        <HostDrawer
          key={editing.id ?? "new"}
          host={editing}
          keys={keys}
          onClose={() => setEditing(null)}
          onSave={async (body: ReturnType<typeof normalizeHostBody>) => {
            if (editing.id) await api(`/api/admin/hosts/${editing.id}`, { method: "PUT", body });
            else await api("/api/admin/hosts", { method: "POST", body });
            setEditing(null);
            await onReload();
            message.success("已保存");
          }}
        />
      ) : null}
    </section>
  );
}

function renderGroup(
  node: GroupNode,
  depth: number,
  isOpen: (path: string) => boolean,
  toggle: (path: string) => void,
  createGroup: (path: string) => void,
  setEditing: (host: HostDraft) => void,
  keys: APIKey[],
  onReload: () => Promise<void>,
  onGroups: (groups: string[], hosts?: Host[]) => void,
  modal: ReturnType<typeof App.useApp>["modal"],
): ReactNode[] {
  const open = isOpen(node.path);
  const rows: ReactNode[] = [
    <div className="ncc-tree-row ncc-tree-group" key={node.path} style={{ ["--depth" as string]: depth }}>
      <button className="ncc-tree-toggle" type="button" onClick={() => toggle(node.path)}>
        {open ? "▾" : "▸"}
      </button>
      <div className="ncc-tree-main" onClick={() => toggle(node.path)} style={{ cursor: "pointer" }}>
        <strong>{node.name}</strong>
        <div className="ncc-tree-meta">{node.totalHostCount} 台 · {node.path}</div>
      </div>
      <Space size={6} wrap>
        <Button size="small" onClick={() => createGroup(node.path)}>子分组</Button>
        <Button size="small" onClick={() => setEditing(emptyHost(node.path))}>添加主机</Button>
        <Button
          size="small"
          danger
          onClick={() => {
            modal.confirm({
              title: `删除分组 ${node.path}？`,
              content: "其中的主机会移到上一级。",
              okText: "删除",
              okButtonProps: { danger: true },
              onOk: async () => {
                const result = await api<{ groups: string[]; hosts?: Host[] }>("/api/admin/groups/delete", {
                  method: "POST",
                  body: { path: node.path },
                });
                onGroups(result.groups || [], result.hosts);
              },
            });
          }}
        >
          删除
        </Button>
        <Button
          size="small"
          danger
          onClick={() => {
            modal.confirm({
              title: `删除分组 ${node.path} 及其下 ${node.totalHostCount || 0} 台主机？`,
              content: "此操作无法撤销。",
              okText: "删除组及主机",
              okButtonProps: { danger: true },
              onOk: async () => {
                const result = await api<{ groups: string[]; hosts?: Host[] }>("/api/admin/groups/delete", {
                  method: "POST",
                  body: { path: node.path, deleteHosts: true },
                });
                onGroups(result.groups || [], result.hosts);
              },
            });
          }}
        >
          删除组及主机
        </Button>
      </Space>
    </div>,
  ];
  if (!open) return rows;
  for (const host of node.hosts) {
    rows.push(
      <HostRow
        key={host.id}
        host={host}
        depth={depth + 1}
        keys={keys}
        onEdit={() => setEditing(cloneHostForEdit(host))}
        onReload={onReload}
      />,
    );
  }
  const children = Object.values(node.children).sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
  for (const child of children) {
    rows.push(...renderGroup(child, depth + 1, isOpen, toggle, createGroup, setEditing, keys, onReload, onGroups, modal));
  }
  return rows;
}

function HostRow({
  host,
  depth,
  keys,
  onEdit,
  onReload,
}: {
  host: Host;
  depth: number;
  keys: APIKey[];
  onEdit: () => void;
  onReload: () => Promise<void>;
}) {
  const { modal, message } = App.useApp();
  return (
    <div className="ncc-tree-row" style={{ ["--depth" as string]: depth }}>
      <span />
      <div className="ncc-tree-main">
        <span className="host-label">{host.label}</span>
        <div className="ncc-tree-meta mono">
          {host.hostname}:{host.port}
          {host.username ? ` · ${host.username}` : ""}
          {` · ${String(host.protocol || "ssh").toUpperCase()}`}
          {` · ${hostAuthLabel(host)}`}
          {` · ${hostVisibilityLabel(host, keys)}`}
        </div>
        <div>
          {(host.tags || []).map((tag) => (
            <Tag key={tag} style={{ marginTop: 4 }}>{tag}</Tag>
          ))}
        </div>
      </div>
      <Space>
        <Button size="small" onClick={onEdit}>编辑</Button>
        <Button
          size="small"
          danger
          onClick={() => {
            modal.confirm({
              title: `删除主机 ${host.label}？`,
              okText: "删除",
              okButtonProps: { danger: true },
              onOk: async () => {
                await api(`/api/admin/hosts/${host.id}`, { method: "DELETE" });
                await onReload();
                message.success("已删除");
              },
            });
          }}
        >
          删除
        </Button>
      </Space>
    </div>
  );
}
