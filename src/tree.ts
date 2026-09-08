import type { GroupNode, Host } from "./types";

export function hostListMatches(host: Host, q: string): boolean {
  if (!q) return true;
  return [host.label, host.hostname, host.username, host.group, ...(host.tags || [])]
    .join(" ")
    .toLowerCase()
    .includes(q);
}

export function buildGroupTree(hosts: Host[], customGroups: string[]): {
  tree: GroupNode[];
  ungrouped: Host[];
} {
  const root: Record<string, GroupNode> = {};
  const insertPath = (path: string, host?: Host) => {
    const parts = String(path || "")
      .split("/")
      .map((part) => part.trim())
      .filter(Boolean);
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
  const ungrouped: Host[] = [];
  for (const host of hosts) {
    if (host.group && String(host.group).trim()) insertPath(host.group, host);
    else ungrouped.push(host);
  }
  const countHosts = (node: GroupNode): number => {
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

export function collectGroupPaths(nodes: GroupNode[]): string[] {
  const paths: string[] = [];
  const walk = (node: GroupNode) => {
    paths.push(node.path);
    for (const child of Object.values(node.children)) walk(child);
  };
  for (const node of nodes) walk(node);
  return paths;
}

export function ancestorPaths(path: string): string[] {
  return path
    .split("/")
    .map((_, index, parts) => parts.slice(0, index + 1).join("/"));
}
