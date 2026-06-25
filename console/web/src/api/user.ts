import type { CreateUserRequest, UpdateUserRequest, User } from "./schemas";

export const USER_ENDPOINTS = {
  list: "/jmate/console/users",
  create: "/jmate/console/users",
  update: (id: string) => `/jmate/console/users/${id}`,
  delete: (id: string) => `/jmate/console/users/${id}`,
} as const;

export type { CreateUserRequest, UpdateUserRequest, User };
