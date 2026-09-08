import { Alert, Button, Form, Input, Typography } from "antd";
import { useState } from "react";
import { api } from "../api";
import { noAuto } from "../theme";
import type { Admin, Settings } from "../types";

type Props = {
  mode: "setup" | "login";
  onReady: (admin: Admin, settings: Settings) => void;
};

export function AuthPage({ mode, onReady }: Props) {
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  return (
    <div className="ncc-auth">
      <section className="ncc-auth-hero">
        <div className="ncc-brand">
          <b>Netcatty Center</b>
          <span>运维服务器组织中心</span>
        </div>
        <h1>主机目录由这里下发。</h1>
        <p>管理员在此维护主机、登录密码和密钥。Netcatty 客户端用「地址 + 密钥」拉取目录后即可连接。</p>
      </section>
      <section className="ncc-auth-form">
        <Form
          className="ncc-auth-card"
          layout="vertical"
          autoComplete="off"
          onFinish={async (values: { username: string; password: string }) => {
            setError("");
            setLoading(true);
            try {
              const path = mode === "setup" ? "/api/admin/setup" : "/api/admin/login";
              const result = await api<{ admin: Admin; settings: Settings }>(path, {
                method: "POST",
                body: values,
              });
              onReady(result.admin, result.settings);
            } catch (err) {
              setError(err instanceof Error ? err.message : "请求失败");
            } finally {
              setLoading(false);
            }
          }}
        >
          <Typography.Title level={3}>{mode === "setup" ? "初始化管理员" : "管理员登录"}</Typography.Title>
          <Typography.Paragraph type="secondary">
            {mode === "setup"
              ? "首次启动需要创建一个管理员账号。之后用该账号维护主机目录，并签发客户端密钥。"
              : "使用组织中心管理员账号登录。"}
          </Typography.Paragraph>
          {error ? <Alert type="error" message={error} style={{ marginBottom: 16 }} /> : null}
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input {...noAuto} />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            rules={[
              { required: true, message: "请输入密码" },
              ...(mode === "setup" ? [{ min: 8, message: "密码至少 8 位" }] : []),
            ]}
          >
            <Input.Password {...noAuto} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            {mode === "setup" ? "创建管理员并进入" : "登录"}
          </Button>
        </Form>
      </section>
    </div>
  );
}
