import { App, Button, Card, Input, Typography } from "antd";
import { useState } from "react";
import { api } from "../api";
import { noAuto } from "../theme";
import type { Admin, Settings } from "../types";

type Props = {
  settings: Settings;
  admin: Admin;
  onSettings: (settings: Settings) => void;
};

export function SettingsPage({ settings, admin, onSettings }: Props) {
  const { message } = App.useApp();
  const [centerName, setCenterName] = useState(settings.centerName);
  const [saving, setSaving] = useState(false);
  const origin = window.location.origin;

  return (
    <section className="ncc-page">
      <div className="ncc-page-head">
        <div>
          <h1>组织设置</h1>
          <p>客户端稍后填写的地址就是当前站点 URL。</p>
        </div>
      </div>

      <Card style={{ maxWidth: 560, marginBottom: 16 }}>
        <form
          autoComplete="off"
          onSubmit={async (event) => {
            event.preventDefault();
            setSaving(true);
            try {
              const result = await api<{ settings: Settings }>("/api/admin/settings", {
                method: "PUT",
                body: { centerName },
              });
              onSettings(result.settings);
              message.success("已保存");
            } finally {
              setSaving(false);
            }
          }}
          style={{ display: "flex", flexDirection: "column", gap: 12 }}
        >
          <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <span className="ncc-hint">组织中心名称</span>
            <Input {...noAuto} required value={centerName} onChange={(e) => setCenterName(e.target.value)} />
          </label>
          <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <span className="ncc-hint">客户端地址</span>
            <Input className="mono" {...noAuto} readOnly value={origin} />
          </label>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            健康检查：<span className="mono">{origin}/api/v1/health</span>
            <br />
            目录接口：<span className="mono">{origin}/api/v1/catalog</span>
          </Typography.Paragraph>
          <Button type="primary" htmlType="submit" loading={saving} style={{ width: "fit-content" }}>
            保存
          </Button>
        </form>
      </Card>

      <Card style={{ maxWidth: 560 }} title="管理员账号">
        <p className="ncc-hint">用户名和密码通过启动参数或环境变量指定，不会写入数据库。修改后重启进程即可生效。</p>
        <p className="ncc-hint">
          启动参数：<span className="mono">--admin-user</span> / <span className="mono">--admin-password</span>
        </p>
        <p className="ncc-hint">
          环境变量：<span className="mono">NCC_ADMIN_USER</span> / <span className="mono">NCC_ADMIN_PASSWORD</span>
        </p>
        <p className="ncc-hint">
          当前登录用户：<span className="mono">{admin.username || "—"}</span>
        </p>
      </Card>
    </section>
  );
}
