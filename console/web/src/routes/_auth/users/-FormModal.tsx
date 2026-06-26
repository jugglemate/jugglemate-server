import { Form, Input, Select } from "antd";
import type { FormInstance } from "antd/es/form";
import type { CreateUserRequest, User } from "@/api/schemas";
import { useTranslation } from "react-i18next";
import { useUserRoleOptions } from "@/hooks/useUserRoleOptions";
import { BaseFormModal } from "@/components/FormModal";

export type FormModalProps = {
  open: boolean;
  editingUser: User | null;
  form: FormInstance<CreateUserRequest>;
  confirmLoading: boolean;
  onCancel: () => void;
  onFinish: (values: CreateUserRequest) => void;
};

export function FormModal({
  open,
  editingUser,
  form,
  confirmLoading,
  onCancel,
  onFinish,
}: FormModalProps) {
  const { t } = useTranslation();
  const roleOptions = useUserRoleOptions();

  return (
    <BaseFormModal<CreateUserRequest>
      open={open}
      title={editingUser ? t("users.editUser") : t("users.newUser")}
      okText={t("common.ok")}
      cancelText={t("common.cancel")}
      form={form}
      confirmLoading={confirmLoading}
      onCancel={onCancel}
      onFinish={onFinish}
    >
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
        <Input disabled={Boolean(editingUser)} />
      </Form.Item>
      {!editingUser ? (
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
      <Form.Item
        name="role"
        label={t("users.rolePlaceholder")}
        initialValue="customer"
        rules={[{ required: true, message: t("users.roleRequired") }]}
      >
        <Select options={[...roleOptions]} />
      </Form.Item>
      <Form.Item name="email" label={t("users.email")}>
        <Input />
      </Form.Item>
    </BaseFormModal>
  );
}
