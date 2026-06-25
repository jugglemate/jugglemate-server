import { Form, Input, Select } from "antd";
import type { FormInstance } from "antd/es/form";
import type { CreateUserRequest, User } from "@/api/schemas";
import { USER_ROLE_OPTIONS } from "@/api/schemas";
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
  return (
    <BaseFormModal<CreateUserRequest>
      open={open}
      title={editingUser ? "Edit User" : "New User"}
      okText="OK"
      cancelText="Cancel"
      form={form}
      confirmLoading={confirmLoading}
      onCancel={onCancel}
      onFinish={onFinish}
    >
      <Form.Item
        name="username"
        label="Username"
        rules={[
          { required: true, message: "Please enter username" },
          {
            pattern: /^[a-zA-Z0-9]{6,20}$/,
            message: "Username must be 6-20 letters or digits",
          },
        ]}
      >
        <Input disabled={Boolean(editingUser)} />
      </Form.Item>
      {!editingUser ? (
        <Form.Item
          name="password"
          label="Password"
          rules={[
            { required: true, message: "Please enter password" },
            { min: 6, message: "Password must be at least 6 characters" },
          ]}
        >
          <Input.Password />
        </Form.Item>
      ) : null}
      <Form.Item
        name="role"
        label="Role"
        initialValue="customer"
        rules={[{ required: true, message: "Please select a role" }]}
      >
        <Select options={[...USER_ROLE_OPTIONS]} />
      </Form.Item>
      <Form.Item name="email" label="Email">
        <Input />
      </Form.Item>
    </BaseFormModal>
  );
}
