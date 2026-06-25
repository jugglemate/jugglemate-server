import type { CreateUserRequest, UpdateUserRequest, User } from "./schemas";

export const USER_ENDPOINTS = {
  list: "/api/users",
  create: "/api/users",
  update: (id: string) => `/api/users/${id}`,
  delete: (id: string) => `/api/users/${id}`,
} as const;

export type { CreateUserRequest, UpdateUserRequest, User };
