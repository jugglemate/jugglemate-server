import { Link, createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { Form, Input, Button, Card, App, theme, Typography, Flex } from "antd";
import type { CSSProperties } from "react";
import { useMutation } from "@tanstack/react-query";
import { httpClient } from "@/utils/http";
import { useAuthStore } from "@/stores/auth";
import { AUTH_ENDPOINTS } from "@/api/auth";
import {
  JmateLoginResponseSchema,
  JmateRegisterRequestSchema,
  RegisterFormSchema,
  type RegisterFormValues,
} from "@/api/schemas";
import { applyLoginSession } from "@/utils/session";
import { getAppKey } from "@/utils/appkey";
import { APP_BRAND_NAME, APP_FAVICON_SRC } from "@/utils/constants";
import { getRegisterErrorMessage } from "@/utils/authErrors";
import { Aurora } from "@/components/Aurora";
import { AppFooter } from "@/components/Layout/AppFooter";
import "../login/index.css";

export const Route = createFileRoute("/register/")({
  beforeLoad: () => {
    const { isAuthenticated } = useAuthStore.getState();
    if (isAuthenticated) {
      throw redirect({ to: "/dashboard" });
    }
  },
  component: RegisterPage,
});

function RegisterPage() {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const { token } = theme.useToken();

  const registerMutation = useMutation({
    mutationFn: async (values: RegisterFormValues) => {
      if (!getAppKey()) {
        throw new Error("缺少 appkey，请先通过 ?appkey=xxx 访问控制台");
      }
      const parsed = RegisterFormSchema.parse(values);
      const body = JmateRegisterRequestSchema.parse({
        account: parsed.account,
        password: parsed.password,
      });
      const data = await httpClient.post(AUTH_ENDPOINTS.register, body);
      const login = JmateLoginResponseSchema.parse(data);
      applyLoginSession(login);
    },
    onSuccess: () => {
      message.success("注册成功");
      void navigate({ to: "/dashboard" });
    },
    onError: (err) => {
      message.error(getRegisterErrorMessage(err));
    },
  });

  const shellStyle: CSSProperties = {
    position: "relative",
    isolation: "isolate",
    minHeight: "100vh",
    backgroundColor: token.colorBgLayout,
    color: token.colorText,
  };

  const contentStyle: CSSProperties = {
    position: "relative",
    zIndex: 1,
    flex: "1 1 0%",
    minWidth: 0,
    minHeight: 0,
    overflow: "hidden",
  };

  const cardStyle: CSSProperties = {
    width: "100%",
    maxWidth: 384,
    background: `color-mix(in srgb, ${token.colorBgContainer} 44%, transparent)`,
    backdropFilter: "blur(18px) saturate(1.35)",
    WebkitBackdropFilter: "blur(18px) saturate(1.35)",
    borderColor: token.colorBorderSecondary,
    boxShadow: token.boxShadow,
    ["--login-card-fallback-bg" as string]: token.colorBgElevated,
  };

  return (
    <Flex vertical style={shellStyle}>
      <Aurora />
      <Flex vertical style={contentStyle}>
        <Flex
          flex={1}
          align="center"
          justify="center"
          style={{ padding: token.padding, minHeight: 0 }}
        >
          <Card
            className="login-page__card"
            style={cardStyle}
            styles={{
              body: { padding: token.paddingLG, background: "transparent" },
            }}
          >
            <Flex
              align="center"
              justify="center"
              gap={token.margin}
              wrap="wrap"
              style={{ marginBottom: token.marginLG }}
            >
              <img
                src={APP_FAVICON_SRC}
                alt=""
                width={32}
                height={32}
                draggable={false}
                style={{ display: "block", flexShrink: 0 }}
              />
              <Typography.Title
                level={3}
                style={{
                  margin: 0,
                  fontWeight: "bold",
                  letterSpacing: "-0.025em",
                  lineHeight: 1.2,
                  textTransform: "uppercase",
                }}
              >
                {APP_BRAND_NAME}
              </Typography.Title>
            </Flex>

            <Form
              layout="vertical"
              onFinish={(values) => {
                registerMutation.mutate(RegisterFormSchema.parse(values));
              }}
              validateTrigger="onBlur"
              requiredMark={false}
            >
              <Form.Item
                name="account"
                label={<span style={{ fontWeight: 500 }}>账号</span>}
                rules={[
                  { required: true, message: "请输入账号" },
                  {
                    pattern: /^[a-zA-Z0-9]{6,20}$/,
                    message: "账号为 6-20 位字母或数字",
                  },
                ]}
              >
                <Input placeholder="6-20 位字母或数字" size="large" />
              </Form.Item>

              <Form.Item
                name="password"
                label={<span style={{ fontWeight: 500 }}>密码</span>}
                rules={[
                  { required: true, message: "请输入密码" },
                  { min: 6, message: "密码至少 6 位" },
                ]}
              >
                <Input.Password placeholder="至少 6 位" size="large" />
              </Form.Item>

              <Form.Item
                name="confirmPassword"
                label={<span style={{ fontWeight: 500 }}>确认密码</span>}
                dependencies={["password"]}
                rules={[
                  { required: true, message: "请确认密码" },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue("password") === value) {
                        return Promise.resolve();
                      }
                      return Promise.reject(new Error("两次输入的密码不一致"));
                    },
                  }),
                ]}
              >
                <Input.Password placeholder="再次输入密码" size="large" />
              </Form.Item>

              <Form.Item style={{ marginBottom: 0, marginTop: token.marginLG }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={registerMutation.isPending}
                  block
                  size="large"
                >
                  注册
                </Button>
              </Form.Item>

              <Flex justify="center" style={{ marginTop: token.margin }}>
                <Typography.Text type="secondary">
                  已有账号？{" "}
                  <Link to="/login" style={{ color: token.colorPrimary }}>
                    去登录
                  </Link>
                </Typography.Text>
              </Flex>
            </Form>
          </Card>
        </Flex>
        <Flex
          vertical
          align="center"
          style={{
            padding: `${token.paddingSM}px ${token.padding}px`,
            textAlign: "center",
          }}
        >
          <AppFooter />
        </Flex>
      </Flex>
    </Flex>
  );
}
