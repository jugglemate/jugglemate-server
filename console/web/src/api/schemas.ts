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

export const CreateUserRequestSchema = z.object({
  username: z.string().min(1),
  email: createUserEmailSchema,
  roles: z.array(z.string()).min(1),
});

export type CreateUserRequest = z.infer<typeof CreateUserRequestSchema>;

export const UpdateUserRequestSchema = CreateUserRequestSchema.partial();

export type UpdateUserRequest = z.infer<typeof UpdateUserRequestSchema>;
