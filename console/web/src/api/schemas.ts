import { z } from "zod/v4";

export const UserSchema = z.object({
  id: z.string(),
  username: z.string(),
  avatar: z.string().nullable(),
  email: z.string().nullable(),
  roles: z.array(z.string()),
  permissions: z.array(z.string()),
});

export type User = z.infer<typeof UserSchema>;

/** GET `/api/auth/user` body (no `permissions`; load via `/api/auth/permissions`). */
export const AuthUserResponseSchema = UserSchema.omit({ permissions: true });
export type AuthUserResponse = z.infer<typeof AuthUserResponseSchema>;

export const AuthTokensSchema = z.object({
  accessToken: z.string().min(1),
  refreshToken: z.string().optional().default(""),
});

export type AuthTokens = z.infer<typeof AuthTokensSchema>;

export const JmateLoginRequestSchema = z.object({
  account: z.string().min(1),
  password: z.string().min(1),
});

export type JmateLoginRequest = z.infer<typeof JmateLoginRequestSchema>;

export const JmateLoginResponseSchema = z.object({
  user_id: z.string(),
  nickname: z.string(),
  avatar: z.string(),
  role: z.number().int(),
  authorization: z.string().min(1),
  im_token: z.string().optional().default(""),
});

export type JmateLoginResponse = z.infer<typeof JmateLoginResponseSchema>;

export const JmateRegisterRequestSchema = JmateLoginRequestSchema;

export type JmateRegisterRequest = JmateLoginRequest;

export const LoginRequestSchema = JmateLoginRequestSchema;

export type LoginRequest = JmateLoginRequest;

export const RefreshTokenRequestSchema = z.object({
  refreshToken: z.string().min(1),
});

export type RefreshTokenRequest = z.infer<typeof RefreshTokenRequestSchema>;

/** Register form fields (maps to `/jmate/user/register` body). */
export const RegisterFormSchema = z
  .object({
    account: z
      .string()
      .min(1, "请输入账号")
      .regex(/^[a-zA-Z0-9]{6,20}$/, "账号为 6-20 位字母或数字"),
    password: z.string().min(6, "密码至少 6 位"),
    confirmPassword: z.string().min(6, "请确认密码"),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "两次输入的密码不一致",
    path: ["confirmPassword"],
  });

export type RegisterFormValues = z.infer<typeof RegisterFormSchema>;

/** @deprecated Use RegisterFormSchema for the register page form. */
export const RegisterRequestSchema = z.object({
  account: z.string().min(1),
  password: z.string().min(6),
});

export type RegisterRequest = z.infer<typeof RegisterRequestSchema>;

export const PermissionsListSchema = z.array(z.string());

export type PermissionsList = z.infer<typeof PermissionsListSchema>;

const BaseMenuNodeSchema = z.object({
  id: z.string(),
  name: z.string(),
  icon: z.string().nullable(),
  permissions: z.array(z.string()).nullable(),
  sort: z.int(),
  hidden: z.boolean(),
});

export type MenuItem =
  | (z.infer<typeof BaseMenuNodeSchema> & {
      kind: "item";
      path: string;
      children: MenuItem[] | null;
    })
  | (z.infer<typeof BaseMenuNodeSchema> & {
      kind: "group";
      path: null;
      children: MenuItem[];
    });

export const MenuItemSchema: z.ZodType<MenuItem> = z.lazy(() =>
  z.discriminatedUnion("kind", [
    BaseMenuNodeSchema.extend({
      kind: z.literal("item"),
      path: z.string(),
      children: z.array(MenuItemSchema).nullable(),
    }),
    BaseMenuNodeSchema.extend({
      kind: z.literal("group"),
      path: z.null(),
      children: z.array(MenuItemSchema),
    }),
  ]),
);

export function ApiResponseSchema<T extends z.ZodTypeAny>(dataSchema: T) {
  return z.object({
    code: z.int(),
    data: dataSchema,
    message: z.string(),
  });
}

export type ApiResponse<T> = {
  code: number;
  data: T;
  message: string;
};

export function PaginatedResponseSchema<T extends z.ZodTypeAny>(itemSchema: T) {
  return z.object({
    code: z.int(),
    data: z.object({
      list: z.array(itemSchema),
      total: z.int(),
    }),
    message: z.string(),
  });
}

export type PaginatedData<T> = {
  list: T[];
  total: number;
};

export const SearchParamsSchema = z.object({
  page: z.number().int().positive().catch(1),
  pageSize: z.number().int().positive().catch(10),
  sortField: z.string().nullable().catch(null),
  sortOrder: z.enum(["ascend", "descend"]).nullable().catch(null),
});

export type SearchParams = z.infer<typeof SearchParamsSchema>;

/** Ant Design Input submits `""` when empty; treat as null for optional email */
const createUserEmailSchema = z
  .union([z.string(), z.null(), z.undefined()])
  .transform((v) => {
    if (v == null) return null;
    const t = String(v).trim();
    return t === "" ? null : t;
  })
  .pipe(z.union([z.string().email(), z.null()]));

export const UserRoleValueSchema = z.enum(["admin", "customer"]);

export type UserRoleValue = z.infer<typeof UserRoleValueSchema>;

export const USER_ROLE_OPTIONS = [
  { label: "Admin", value: "admin" as const },
  { label: "Customer", value: "customer" as const },
] as const;

export const CreateUserRequestSchema = z.object({
  username: z
    .string()
    .min(1)
    .regex(/^[a-zA-Z0-9]{6,20}$/, "Username must be 6-20 letters or digits"),
  password: z.string().min(6, "Password must be at least 6 characters").optional(),
  email: createUserEmailSchema,
  role: UserRoleValueSchema,
});

export type CreateUserRequest = z.infer<typeof CreateUserRequestSchema>;

export const UpdateUserRequestSchema = CreateUserRequestSchema.omit({ password: true }).partial();

export type UpdateUserRequest = z.infer<typeof UpdateUserRequestSchema>;

export function rolesToFormRole(roles: string[]): UserRoleValue {
  return roles.includes("admin") ? "admin" : "customer";
}

export function roleNumberToRoles(role: number): UserRoleValue[] {
  return role === 2 ? ["admin"] : ["customer"];
}

export function toCreateUserApiBody(values: CreateUserRequest) {
  return {
    username: values.username,
    password: values.password ?? "",
    email: values.email ?? "",
    roles: [values.role],
  };
}

export function toUpdateUserApiBody(
  values: Pick<CreateUserRequest, "username" | "email" | "role">,
) {
  return {
    username: values.username,
    email: values.email ?? "",
    roles: [values.role],
  };
}

export const TelegramChannelConfigSchema = z.object({
  bot_name: z.string(),
  bot_token: z.string().optional().default(""),
});

export type TelegramChannelConfig = z.infer<typeof TelegramChannelConfigSchema>;

export const WidgetChannelConfigSchema = z.object({
  welcome_message: z.string().catch(""),
});

export type WidgetChannelConfig = z.infer<typeof WidgetChannelConfigSchema>;

const InboxBaseSchema = z.object({
  id: z.string(),
  name: z.string(),
  member_count: z.number().int().nonnegative(),
  created_time: z.number().int().catch(0),
  updated_time: z.number().int().catch(0),
});

export const TelegramInboxSchema = InboxBaseSchema.extend({
  channel_type: z.literal("telegram"),
  channel_conf: TelegramChannelConfigSchema,
});

export const WidgetInboxSchema = InboxBaseSchema.extend({
  channel_type: z.literal("widget"),
  channel_conf: WidgetChannelConfigSchema,
});

export const InboxSchema = z.discriminatedUnion("channel_type", [
  TelegramInboxSchema,
  WidgetInboxSchema,
]);

export type Inbox = z.infer<typeof InboxSchema>;

export const CreateTelegramInboxRequestSchema = z.object({
  name: z.string().trim().min(1, "Please enter inbox name"),
  bot_name: z.string().trim().min(1, "Please enter bot name"),
  bot_token: z.string().trim().min(1, "Please enter bot token"),
});

export type CreateTelegramInboxRequest = z.infer<typeof CreateTelegramInboxRequestSchema>;

export const CreateWidgetInboxRequestSchema = z.object({
  name: z.string().trim().min(1, "Please enter inbox name"),
  welcome_message: z
    .string()
    .trim()
    .max(500, "Welcome message must be at most 500 characters")
    .catch(""),
});

export type CreateWidgetInboxRequest = z.infer<typeof CreateWidgetInboxRequestSchema>;

export const InboxMemberSchema = z.object({
  id: z.string(),
  username: z.string(),
  avatar: z.string().nullable().catch(null),
  email: z.string().nullable().catch(null),
});

export type InboxMember = z.infer<typeof InboxMemberSchema>;

export const InboxMembersResponseSchema = z.object({
  list: z.array(InboxMemberSchema),
});

export type InboxMembersResponse = z.infer<typeof InboxMembersResponseSchema>;
