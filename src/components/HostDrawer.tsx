import { Button, Checkbox, Drawer, Input, Select, Space, Typography } from "antd";
import { type ReactNode, useMemo, useState } from "react";
import { hostStartupRules, normalizeHostBody } from "../hosts";
import { noAuto } from "../theme";
import type { APIKey, HostDraft } from "../types";

type Props = {
  host: HostDraft;
  keys: APIKey[];
  onClose: () => void;
  onSave: (body: ReturnType<typeof normalizeHostBody>) => Promise<void>;
};

export function HostDrawer({ host: initial, keys, onClose, onSave }: Props) {
  const [host, setHost] = useState<HostDraft>(initial);
  const [saving, setSaving] = useState(false);
  const [keyFilter, setKeyFilter] = useState("");
  const isNew = !host.id;
  const wide = host.visibility === "keys";
  const rules = hostStartupRules(host);
  const tagsValue = Array.isArray(host.tags) ? host.tags.join(", ") : host.tags;

  const visibleKeys = useMemo(() => {
    const q = keyFilter.trim().toLowerCase();
    if (!q) return keys;
    return keys.filter((key) =>
      `${key.name} ${key.keyPrefix || ""} ${key.permission} ${key.revokedAt ? "已吊销" : "有效"}`
        .toLowerCase()
        .includes(q),
    );
  }, [keys, keyFilter]);

  const patch = (partial: Partial<HostDraft>) => setHost((prev) => ({ ...prev, ...partial }));

  return (
    <Drawer
      title={isNew ? "添加主机" : "编辑主机"}
      open
      width={wide ? 920 : 520}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button
            type="primary"
            loading={saving}
            onClick={async () => {
              setSaving(true);
              try {
                await onSave(normalizeHostBody(host));
              } finally {
                setSaving(false);
              }
            }}
          >
            保存
          </Button>
        </Space>
      }
    >
      <form autoComplete="off" onSubmit={(e) => e.preventDefault()}>
        <div style={{ display: "grid", gridTemplateColumns: wide ? "1fr 320px" : "1fr", gap: 24 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <Field label="显示名称">
              <Input {...noAuto} required value={host.label} onChange={(e) => patch({ label: e.target.value })} />
            </Field>
            <div className="grid-2" style={{ display: "grid", gridTemplateColumns: "1fr 120px", gap: 12 }}>
              <Field label="主机名 / IP">
                <Input className="mono" {...noAuto} required value={host.hostname} onChange={(e) => patch({ hostname: e.target.value })} />
              </Field>
              <Field label="端口">
                <Input {...noAuto} type="number" value={host.port} onChange={(e) => patch({ port: Number(e.target.value) })} />
              </Field>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <Field label="用户名">
                <Input className="mono" {...noAuto} value={host.username} onChange={(e) => patch({ username: e.target.value })} />
              </Field>
              <Field label="分组（用 / 嵌套）">
                <Input {...noAuto} placeholder="production/web" value={host.group} onChange={(e) => patch({ group: e.target.value })} />
              </Field>
            </div>
            <Field label="标签（逗号分隔）">
              <Input {...noAuto} value={tagsValue} onChange={(e) => patch({ tags: e.target.value })} />
            </Field>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12 }}>
              <Field label="协议">
                <Select
                  value={host.protocol}
                  options={[{ value: "ssh", label: "SSH" }, { value: "telnet", label: "Telnet" }]}
                  onChange={(value) => patch({ protocol: value })}
                />
              </Field>
              <Field label="系统">
                <Select
                  value={host.os}
                  options={[
                    { value: "linux", label: "Linux" },
                    { value: "windows", label: "Windows" },
                    { value: "macos", label: "macOS" },
                  ]}
                  onChange={(value) => patch({ os: value })}
                />
              </Field>
              <Field label="设备类型">
                <Select
                  value={host.deviceType}
                  options={[
                    { value: "general", label: "通用服务器" },
                    { value: "network", label: "网络设备" },
                  ]}
                  onChange={(value) => patch({ deviceType: value })}
                />
              </Field>
            </div>
            <Field label="备注">
              <Input.TextArea {...noAuto} rows={3} value={host.notes} onChange={(e) => patch({ notes: e.target.value })} />
            </Field>
            <Field label="登录密码">
              <Input.Password {...noAuto} value={host.password} onChange={(e) => patch({ password: e.target.value })} />
            </Field>
            <Field label="私钥（OpenSSH / PEM）">
              <Input.TextArea
                className="mono"
                {...noAuto}
                rows={4}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                value={host.privateKey}
                onChange={(e) => patch({ privateKey: e.target.value })}
              />
            </Field>
            <Field label="私钥口令（可选）">
              <Input.Password {...noAuto} value={host.passphrase} onChange={(e) => patch({ passphrase: e.target.value })} />
            </Field>
            <Field label="连接后发送方式">
              <Select
                value={host.startupCommandRunMode || "paste"}
                options={[
                  { value: "paste", label: "一次性发送" },
                  { value: "lineDelay", label: "逐行发送" },
                  { value: "rules", label: "规则模式（Expect / Send）" },
                ]}
                onChange={(value) => {
                  patch({
                    startupCommandRunMode: value,
                    startupCommandRules:
                      value === "rules" && rules.length === 0 ? [{ expect: "", send: "" }] : host.startupCommandRules,
                  });
                }}
              />
            </Field>
            {host.startupCommandRunMode === "rules" ? (
              <div className="ncc-rules">
                <span className="ncc-hint">Expect / Send 规则</span>
                {rules.map((rule, index) => (
                  <div className="ncc-rule-card" key={index}>
                    <div className="ncc-rule-head">
                      <span>步骤 {index + 1}</span>
                      <Button
                        size="small"
                        onClick={() =>
                          patch({ startupCommandRules: rules.filter((_, i) => i !== index) })
                        }
                      >
                        删除
                      </Button>
                    </div>
                    <Input
                      className="mono"
                      {...noAuto}
                      placeholder="匹配文本（留空则立即发送，例如 password:）"
                      value={rule.expect}
                      onChange={(e) => {
                        const next = rules.map((item, i) => (i === index ? { ...item, expect: e.target.value } : item));
                        patch({ startupCommandRules: next });
                      }}
                    />
                    <Input
                      className="mono"
                      {...noAuto}
                      placeholder="发送内容（例如 ssh user@jump）"
                      value={rule.send}
                      onChange={(e) => {
                        const next = rules.map((item, i) => (i === index ? { ...item, send: e.target.value } : item));
                        patch({ startupCommandRules: next });
                      }}
                    />
                  </div>
                ))}
                <Button onClick={() => patch({ startupCommandRules: [...rules, { expect: "", send: "" }] })}>
                  添加规则
                </Button>
              </div>
            ) : (
              <Field label="启动命令">
                <Input.TextArea
                  className="mono"
                  {...noAuto}
                  rows={3}
                  placeholder="连接后执行的命令（例如：cd /app && ls）"
                  value={host.startupCommand}
                  onChange={(e) => patch({ startupCommand: e.target.value })}
                />
              </Field>
            )}
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {host.startupCommandRunMode === "rules"
                ? "终端出现指定文本后发送对应命令。匹配文本留空则立即发送。可添加多条规则，按顺序跳转主机。规则会随目录下发给客户端。"
                : "SSH 连接建立后将自动执行该命令。规则模式与启动命令互斥。密码、密钥和启动规则都会随目录下发。"}
            </Typography.Paragraph>
            <Field label="目录可见范围">
              <Select
                value={host.visibility || "all"}
                options={[
                  { value: "all", label: "全部可见" },
                  { value: "keys", label: "仅特定密钥列表可见" },
                ]}
                onChange={(value) => patch({ visibility: value })}
              />
            </Field>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {host.visibility === "keys"
                ? "只有勾选的客户端密钥拉取目录时能看到这台主机。未勾选任何密钥则对所有客户端隐藏。"
                : "所有有效客户端密钥都能拉取到这台主机。"}
            </Typography.Paragraph>
          </div>
          {wide ? (
            <aside>
              <Typography.Title level={4} style={{ marginTop: 0 }}>可见密钥</Typography.Title>
              {keys.length === 0 ? (
                <p className="ncc-hint">还没有客户端密钥。请先到「客户端密钥」签发，再勾选可见范围。</p>
              ) : (
                <>
                  <p className="ncc-hint">已选 {(host.visibleKeyIds || []).length} / {keys.length}</p>
                  <Input
                    {...noAuto}
                    placeholder="搜索密钥名称 / 前缀"
                    value={keyFilter}
                    onChange={(e) => setKeyFilter(e.target.value)}
                    style={{ marginBottom: 8 }}
                  />
                  <Space style={{ marginBottom: 8 }}>
                    <Button
                      size="small"
                      onClick={() => patch({ visibleKeyIds: keys.filter((key) => !key.revokedAt).map((key) => key.id) })}
                    >
                      全选有效
                    </Button>
                    <Button size="small" onClick={() => patch({ visibleKeyIds: [] })}>清空</Button>
                  </Space>
                  <div className="ncc-key-list">
                    {visibleKeys.map((key) => {
                      const checked = (host.visibleKeyIds || []).includes(key.id);
                      return (
                        <label className="ncc-key-option" key={key.id}>
                          <Checkbox
                            checked={checked}
                            onChange={(e) => {
                              const next = new Set(host.visibleKeyIds || []);
                              if (e.target.checked) next.add(key.id);
                              else next.delete(key.id);
                              patch({ visibleKeyIds: [...next] });
                            }}
                          />
                          <span>
                            <strong>{key.name}{key.revokedAt ? "（已吊销）" : ""}</strong>
                            <div className="ncc-tree-meta mono">
                              {key.keyPrefix ? `${key.keyPrefix}…` : "—"} · {key.permission === "readwrite" ? "可读可写" : "只读"} · {key.revokedAt ? "已吊销" : "有效"}
                            </div>
                          </span>
                        </label>
                      );
                    })}
                  </div>
                </>
              )}
            </aside>
          ) : null}
        </div>
      </form>
    </Drawer>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      <span className="ncc-hint">{label}</span>
      {children}
    </label>
  );
}
