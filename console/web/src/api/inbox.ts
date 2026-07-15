import { httpClient } from "@/utils/http";
import i18n from "@/i18n";
import {
  CreateJuggleIMInboxRequestSchema,
  CreateTelegramInboxRequestSchema,
  CreateWidgetInboxRequestSchema,
  InboxMembersResponseSchema,
  InboxSchema,
  PaginatedResponseSchema,
  type CreateJuggleIMInboxRequest,
  type CreateTelegramInboxRequest,
  type CreateWidgetInboxRequest,
  type Inbox,
  type InboxMember,
} from "./schemas";

export const INBOX_ENDPOINTS = {
  list: "/jmate/console/inboxes",
  createTelegram: "/jmate/console/inboxes/telegram",
  createJuggleIM: "/jmate/console/inboxes/juggleim",
  createWidget: "/jmate/console/inboxes/widget",
  detail: (id: string) => `/jmate/console/inboxes/${id}`,
  update: (id: string) => `/jmate/console/inboxes/${id}`,
  remove: (id: string) => `/jmate/console/inboxes/${id}`,
  members: (id: string) => `/jmate/console/inboxes/${id}/members`,
} as const;

/** 更新收件箱请求体（名称必填；欢迎语仅对网站挂件渠道生效）。 */
export type UpdateInboxRequest = {
  name: string;
  welcome_message?: string;
};

const InboxListResponseSchema = PaginatedResponseSchema(InboxSchema);

export async function listInboxes(params: { limit: number; offset: number }) {
  const raw = await httpClient.get(INBOX_ENDPOINTS.list, { params });
  return InboxListResponseSchema.shape.data.parse(raw);
}

export async function createTelegramInbox(values: CreateTelegramInboxRequest): Promise<Inbox> {
  const body = CreateTelegramInboxRequestSchema.parse(values);
  const raw = await httpClient.post(INBOX_ENDPOINTS.createTelegram, body);
  return InboxSchema.parse(raw);
}

export async function createJuggleIMInbox(values: CreateJuggleIMInboxRequest): Promise<Inbox> {
  const body = CreateJuggleIMInboxRequestSchema.parse(values);
  const raw = await httpClient.post(INBOX_ENDPOINTS.createJuggleIM, body);
  return InboxSchema.parse(raw);
}

export async function createWidgetInbox(values: CreateWidgetInboxRequest): Promise<Inbox> {
  const body = CreateWidgetInboxRequestSchema.parse(values);
  const raw = await httpClient.post(INBOX_ENDPOINTS.createWidget, body);
  return InboxSchema.parse(raw);
}

export async function getInbox(inboxId: string): Promise<Inbox> {
  const raw = await httpClient.get(INBOX_ENDPOINTS.detail(inboxId));
  return InboxSchema.parse(raw);
}

export async function updateInbox(inboxId: string, values: UpdateInboxRequest): Promise<Inbox> {
  const raw = await httpClient.put(INBOX_ENDPOINTS.update(inboxId), values);
  return InboxSchema.parse(raw);
}

export async function deleteInbox(inboxId: string): Promise<void> {
  await httpClient.delete(INBOX_ENDPOINTS.remove(inboxId));
}

export async function listInboxMembers(inboxId: string): Promise<InboxMember[]> {
  const raw = await httpClient.get(INBOX_ENDPOINTS.members(inboxId));
  return InboxMembersResponseSchema.parse(raw).list;
}

export async function replaceInboxMembers(inboxId: string, userIds: string[]): Promise<void> {
  await httpClient.put(INBOX_ENDPOINTS.members(inboxId), { user_ids: userIds });
}

export function inboxChannelLabel(channelType: Inbox["channel_type"]): string {
  if (channelType === "widget") return i18n.t("inboxes.channelWidget");
  if (channelType === "juggleim") return i18n.t("inboxes.channelJuggleIM");
  return i18n.t("inboxes.channelTelegram");
}

export function inboxSubtitle(inbox: Inbox): string {
  if (inbox.channel_type === "telegram") {
    return `@${inbox.channel_conf.bot_name || "telegram"}`;
  }
  if (inbox.channel_type === "juggleim") {
    return inbox.channel_conf.bot_name || i18n.t("inboxes.channelJuggleIM");
  }
  return inbox.channel_conf.welcome_message || i18n.t("inboxes.widgetSubtitle");
}

export type {
  CreateJuggleIMInboxRequest,
  CreateTelegramInboxRequest,
  CreateWidgetInboxRequest,
  Inbox,
  InboxMember,
};
