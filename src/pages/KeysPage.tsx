import { App, Button, Drawer, Input, Select, Space, Table, Tag, Typography } from "antd";
import { useState } from "react";
import { api, formatTime, permissionLabel } from "../api";
import { noAuto } from "../theme";
import type { APIKey } from "../types";

type Props = {
  keys: APIKey[];
  onReload: () => Promise<void>;
};

export function KeysPage({ keys, onReload }: Props) {
  const { message, modal } = App.useApp();
  const [issuing, setIssuing] = useState<{ name: string; permission: string } | null>(null);
  const [saving, setSaving] = useState(false);

  return (
    <section className="ncc-page">
      <div className="ncc-page-head">
        <div>
          <h1>客户端密钥</h1>
          <p>只读密钥可拉取机器列表并使用分享；可读可写密钥还能改机器列表。新密钥默认只读。</p>
        </div>
        <Button type="primary" onClick={() => setIssuing({ name: "Netcatty 客户端", permission: "read" })}>
          签发密钥
        </Button>
      </div>

      {keys.length === 0 ? (
        <div className="ncc-empty">还没有客户端密钥。签发后即可用 curl 或 Netcatty 客户端拉取目录。</div>
      ) : (
        <Table
          rowKey="id"
          dataSource={keys}
          pagination={false}
          columns={[
            { title: "名称", dataIndex: "name" },
            {
              title: "权限",
              dataIndex: "permission",
              render: (value: string) => <Tag>{permissionLabel(value)}</Tag>,
            },
            {
              title: "密钥",
              render: (_, key) =>
                key.plaintext ? (
                  <Space>
                    <code className="mono ncc-secret">{key.plaintext}</code>
                    <Button
                      size="small"
                      onClick={async () => {
                        await navigator.clipboard.writeText(key.plaintext);
                        message.success("已复制到剪贴板");
                      }}
                    >
                      复制
                    </Button>
                  </Space>
                ) : (
                  <Typography.Text type="secondary">{key.keyPrefix}…（旧密钥无法还原，请重新签发）</Typography.Text>
                ),
            },
            { title: "创建", dataIndex: "createdAt", render: (value: number) => formatTime(value) },
            { title: "最近使用", dataIndex: "lastUsedAt", render: (value: number | null) => formatTime(value) },
            { title: "状态", render: (_, key) => (key.revokedAt ? "已吊销" : "有效") },
            {
              title: "",
              render: (_, key) =>
                key.revokedAt ? (
                  <Space>
                    <Button
                      size="small"
                      onClick={async () => {
                        await api(`/api/admin/keys/${key.id}/restore`, { method: "POST" });
                        message.success("已重新启用密钥");
                        await onReload();
                      }}
                    >
                      重新启用
                    </Button>
                    <Button
                      size="small"
                      danger
                      onClick={() => {
                        modal.confirm({
                          title: `彻底删除密钥 ${key.name}？`,
                          content: "删除后无法恢复，使用该密钥的客户端将无法再拉取目录。",
                          okText: "彻底删除",
                          okButtonProps: { danger: true },
                          onOk: async () => {
                            await api(`/api/admin/keys/${key.id}/delete`, { method: "POST" });
                            message.success("已彻底删除密钥");
                            await onReload();
                          },
                        });
                      }}
                    >
                      彻底删除
                    </Button>
                  </Space>
                ) : (
                  <Space>
                    <Button
                      size="small"
                      onClick={async () => {
                        const next = key.permission === "readwrite" ? "read" : "readwrite";
                        await api(`/api/admin/keys/${key.id}`, { method: "PUT", body: { permission: next } });
                        message.success(`已改为${permissionLabel(next)}`);
                        await onReload();
                      }}
                    >
                      {key.permission === "readwrite" ? "改为只读" : "改为可读可写"}
                    </Button>
                    <Button
                      size="small"
                      danger
                      onClick={() => {
                        modal.confirm({
                          title: "吊销该密钥？",
                          content: "吊销后，使用该密钥的客户端将无法再拉取目录。之后仍可重新启用或彻底删除。",
                          okText: "吊销",
                          okButtonProps: { danger: true },
                          onOk: async () => {
                            await api(`/api/admin/keys/${key.id}`, { method: "DELETE" });
                            await onReload();
                          },
                        });
                      }}
                    >
                      吊销
                    </Button>
                  </Space>
                ),
            },
          ]}
        />
      )}

      <Drawer
        title="签发客户端密钥"
        open={!!issuing}
        onClose={() => setIssuing(null)}
        extra={
          <Space>
            <Button onClick={() => setIssuing(null)}>取消</Button>
            <Button
              type="primary"
              loading={saving}
              onClick={async () => {
                if (!issuing) return;
                setSaving(true);
                try {
                  await api("/api/admin/keys", {
                    method: "POST",
                    body: { name: issuing.name, permission: issuing.permission || "read" },
                  });
                  setIssuing(null);
                  message.success(`已签发${permissionLabel(issuing.permission || "read")}密钥`);
                  await onReload();
                } finally {
                  setSaving(false);
                }
              }}
            >
              签发
            </Button>
          </Space>
        }
      >
        {issuing ? (
          <form autoComplete="off" onSubmit={(e) => e.preventDefault()} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span className="ncc-hint">名称</span>
              <Input {...noAuto} value={issuing.name} onChange={(e) => setIssuing({ ...issuing, name: e.target.value })} />
            </label>
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span className="ncc-hint">权限</span>
              <Select
                value={issuing.permission}
                onChange={(value) => setIssuing({ ...issuing, permission: value })}
                options={[
                  { value: "read", label: "只读（默认）：可拉取机器列表，可开启/加入/关闭分享" },
                  { value: "readwrite", label: "可读可写：还可新增、修改、删除机器列表" },
                ]}
              />
            </label>
            <p className="ncc-hint">只读不会阻止分享。分享的开启、加入和关闭对两种密钥都可用。</p>
          </form>
        ) : null}
      </Drawer>
    </section>
  );
}
