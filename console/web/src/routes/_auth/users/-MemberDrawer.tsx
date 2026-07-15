import {
  Avatar,
  Button,
  Divider,
  Drawer,
  Flex,
  Form,
  Input,
  Select,
  Switch,
  Typography,
  theme,
} from "antd";
import type { FormInstance } from "antd/es/form";
import { UserRoundMinus } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { CreateUserRequest, User } from "@/api/schemas";
import { rolesToFormRole } from "@/api/schemas";
import { useUserRoleOptions } from "@/hooks/useUserRoleOptions";

const { Text } = Typography;

/**
 * 团队成员添加 / 编辑抽屉（右侧滑出）
 * @param open 抽屉是否可见
 * @param editingUser 正在编辑的成员；为 null 时表示新增
 * @param form 由父组件管理的表单实例
 * @param confirmLoading 提交按钮 loading 状态
 * @param onCancel 关闭抽屉回调
 * @param onFinish 表单校验通过后的提交回调
 * @param onRemove 编辑态点击"从团队移除该成员"的回调
 */
export type MemberDrawerProps = {
  open: boolean;
  editingUser: User | null;
  form: FormInstance<CreateUserRequest>;
  confirmLoading: boolean;
  onCancel: () => void;
  onFinish: (values: CreateUserRequest) => void;
  onRemove: (user: User) => void;
};

export function MemberDrawer({
  open,
  editingUser,
  form,
  confirmLoading,
  onCancel,
  onFinish,
  onRemove,
}: MemberDrawerProps) {
  const { t } = useTranslation();
  const roleOptions = useUserRoleOptions();
  const { token } = theme.useToken();
  const isEdit = Boolean(editingUser);

  return (
    <Drawer
      open={open}
      onClose={onCancel}
      size={420}
      /* TIPS: 表单实例由父组件 useForm 创建并通过 setFieldsValue 预填，
         必须 forceRender 保证抽屉未打开时 Form 也已挂载，否则 antd 会告警且预填失效。 */
      forceRender
      title={
        <Flex vertical gap={2}>
          <span>{isEdit ? t("users.editMemberTitle") : t("users.addMemberTitle")}</span>
          {isEdit ? (
            <Text type="secondary" style={{ fontSize: token.fontSizeSM, fontWeight: "normal" }}>
              {t("users.editMemberSubtitle")}
            </Text>
          ) : null}
        </Flex>
      }
      footer={
        <Flex justify="flex-end" gap={token.marginSM}>
          <Button onClick={onCancel}>{t("common.cancel")}</Button>
          <Button type="primary" loading={confirmLoading} onClick={() => form.submit()}>
            {isEdit ? t("users.saveChanges") : t("users.addMember")}
          </Button>
        </Flex>
      }
    >
      {isEdit && editingUser ? (
        <Flex
          align="center"
          gap={token.margin}
          style={{
            background: token.colorPrimaryBg,
            borderRadius: token.borderRadiusLG,
            padding: token.padding,
            marginBottom: token.marginLG,
          }}
        >
          <Avatar size={48} src={(editingUser.avatar ?? "").trim() || undefined} shape="circle">
            {editingUser.username?.[0]?.toUpperCase()}
          </Avatar>
          <Flex vertical gap={2} style={{ minWidth: 0 }}>
            <Text strong ellipsis style={{ fontSize: token.fontSizeLG }}>
              {editingUser.username}
            </Text>
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {rolesToFormRole(editingUser.roles) === "admin"
                ? t("users.role.admin")
                : t("users.role.customer")}
            </Text>
          </Flex>
        </Flex>
      ) : null}

      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="username"
          label={t("users.username")}
          rules={[
            { required: true, message: t("users.usernameRequired") },
            {
              pattern: /^[a-zA-Z0-9]{6,20}$/,
              message: t("users.usernamePattern"),
            },
          ]}
        >
          <Input disabled={isEdit} />
        </Form.Item>
        {!isEdit ? (
          <Form.Item
            name="password"
            label={t("auth.password")}
            rules={[
              { required: true, message: t("users.passwordRequired") },
              { min: 6, message: t("users.passwordMin") },
            ]}
          >
            <Input.Password />
          </Form.Item>
        ) : null}
        <Form.Item name="email" label={t("users.email")}>
          <Input placeholder="john@example.com" />
        </Form.Item>
        <Form.Item
          name="role"
          label={t("users.rolePlaceholder")}
          initialValue="customer"
          rules={[{ required: true, message: t("users.roleRequired") }]}
        >
          <Select options={[...roleOptions]} />
        </Form.Item>

        {/* TIPS: 启用状态开关对齐设计稿；后端暂无成员状态字段，此开关为展示项，
            提交时会被 CreateUserRequestSchema 忽略（schema 不含该字段）。 */}
        <Flex align="center" justify="space-between" style={{ marginTop: token.marginXS }}>
          <Flex vertical gap={2}>
            <span style={{ fontWeight: 500 }}>{t("users.activeStatus")}</span>
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {isEdit ? t("users.activeStatusEditHint") : t("users.activeStatusAddHint")}
            </Text>
          </Flex>
          <Form.Item name="active" valuePropName="checked" initialValue={true} noStyle>
            <Switch />
          </Form.Item>
        </Flex>
      </Form>

      {isEdit && editingUser ? (
        <>
          <Divider />
          <Button
            type="text"
            danger
            icon={<UserRoundMinus size={token.fontSize} />}
            onClick={() => onRemove(editingUser)}
          >
            {t("users.removeMember")}
          </Button>
        </>
      ) : null}
    </Drawer>
  );
}
