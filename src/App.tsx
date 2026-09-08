import { App as AntApp, Button, Layout, Menu, Spin } from "antd";
import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { AuthPage } from "./pages/AuthPage";
import { HostsPage } from "./pages/HostsPage";
import { KeysPage } from "./pages/KeysPage";
import { SettingsPage } from "./pages/SettingsPage";
import type { Admin, APIKey, Host, Settings } from "./types";

type View = "hosts" | "keys" | "settings";

function Shell() {
  const { message } = AntApp.useApp();
  const [booting, setBooting] = useState(true);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [admin, setAdmin] = useState<Admin | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [view, setView] = useState<View>("hosts");
  const [hosts, setHosts] = useState<Host[]>([]);
  const [groups, setGroups] = useState<string[]>([]);
  const [keys, setKeys] = useState<APIKey[]>([]);

  const loadWorkspace = useCallback(async () => {
    const [hostRes, keyRes] = await Promise.all([
      api<{ hosts: Host[]; groups?: string[] }>("/api/admin/hosts"),
      api<{ keys: APIKey[] }>("/api/admin/keys"),
    ]);
    setHosts(hostRes.hosts || []);
    setGroups(hostRes.groups || []);
    setKeys(keyRes.keys || []);
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const status = await api<{ needsSetup: boolean; settings: Settings }>("/api/admin/setup-status");
        setNeedsSetup(status.needsSetup);
        setSettings(status.settings);
        if (!status.needsSetup) {
          try {
            const me = await api<{ admin: Admin; settings: Settings }>("/api/admin/me");
            setAdmin(me.admin);
            setSettings(me.settings);
            await loadWorkspace();
          } catch {
            setAdmin(null);
          }
        }
      } catch (err) {
        message.error(err instanceof Error ? err.message : "无法连接组织中心");
      } finally {
        setBooting(false);
      }
    })();
  }, [loadWorkspace, message]);

  if (booting) {
    return (
      <div style={{ minHeight: "100%", display: "grid", placeItems: "center" }}>
        <Spin size="large" />
      </div>
    );
  }

  if (needsSetup) {
    return (
      <AuthPage
        mode="setup"
        onReady={async (nextAdmin, nextSettings) => {
          setNeedsSetup(false);
          setAdmin(nextAdmin);
          setSettings(nextSettings);
          await loadWorkspace();
        }}
      />
    );
  }

  if (!admin || !settings) {
    return (
      <AuthPage
        mode="login"
        onReady={async (nextAdmin, nextSettings) => {
          setAdmin(nextAdmin);
          setSettings(nextSettings);
          await loadWorkspace();
        }}
      />
    );
  }

  return (
    <Layout className="ncc-shell">
      <Layout.Sider className="ncc-sider" width={220} theme="dark">
        <div className="ncc-brand">
          <b>NCC</b>
          <span>{settings.centerName || "Netcatty Center"}</span>
        </div>
        <Menu
          theme="dark"
          selectedKeys={[view]}
          onClick={({ key }) => setView(key as View)}
          items={[
            { key: "hosts", label: "主机目录" },
            { key: "keys", label: "客户端密钥" },
            { key: "settings", label: "组织设置" },
          ]}
        />
        <div style={{ flex: 1 }} />
        <div className="ncc-sider-foot">
          <div>{admin.username}</div>
          <Button
            onClick={async () => {
              await api("/api/admin/logout", { method: "POST" });
              setAdmin(null);
              setHosts([]);
              setKeys([]);
            }}
          >
            退出
          </Button>
        </div>
      </Layout.Sider>
      <Layout.Content>
        {view === "hosts" ? (
          <HostsPage
            hosts={hosts}
            groups={groups}
            keys={keys}
            onReload={loadWorkspace}
            onGroups={(nextGroups, nextHosts) => {
              setGroups(nextGroups);
              if (nextHosts) setHosts(nextHosts);
            }}
          />
        ) : null}
        {view === "keys" ? <KeysPage keys={keys} onReload={loadWorkspace} /> : null}
        {view === "settings" ? (
          <SettingsPage settings={settings} admin={admin} onSettings={setSettings} />
        ) : null}
      </Layout.Content>
    </Layout>
  );
}

export function App() {
  return (
    <AntApp>
      <Shell />
    </AntApp>
  );
}
