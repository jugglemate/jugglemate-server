import { httpClient } from "@/utils/http";
import i18n from "@/i18n";
import {
  CreateTelegramInboxRequestSchema,
  CreateWidgetInboxRequestSchema,
  InboxMembersResponseSchema,
  InboxSchema,
  PaginatedResponseSchema,
  type CreateTelegramInboxRequest,
  type CreateWidgetInboxRequest,
  type Inbox,
  type InboxMember,
} from "./schemas";

export const INBOX_ENDPOINTS = {
  list: "/jmate/console/inboxes",
  createTelegram: "/jmate/console/inboxes/telegram",
  createWidget: "/jmate/console/inboxes/widget",
  members: (id: string) => `/jmate/console/inboxes/${id}/members`,
} as const;

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

export async function createWidgetInbox(values: CreateWidgetInboxRequest): Promise<Inbox> {
  const body = CreateWidgetInboxRequestSchema.parse(values);
  const raw = await httpClient.post(INBOX_ENDPOINTS.createWidget, body);
  return InboxSchema.parse(raw);
}

export async function listInboxMembers(inboxId: string): Promise<InboxMember[]> {
  const raw = await httpClient.get(INBOX_ENDPOINTS.members(inboxId));
  return InboxMembersResponseSchema.parse(raw).list;
}

export async function replaceInboxMembers(inboxId: string, userIds: string[]): Promise<void> {
  await httpClient.put(INBOX_ENDPOINTS.members(inboxId), { user_ids: userIds });
}

export function inboxChannelLabel(channelType: Inbox["channel_type"]): string {
  return channelType === "widget"
    ? i18n.t("inboxes.channelWidget")
    : i18n.t("inboxes.channelTelegram");
}

export function inboxSubtitle(inbox: Inbox): string {
  if (inbox.channel_type === "telegram") {
    return `@${inbox.channel_conf.bot_name || "telegram"}`;
  }
  return inbox.channel_conf.welcome_message || i18n.t("inboxes.widgetSubtitle");
}

export type { CreateTelegramInboxRequest, CreateWidgetInboxRequest, Inbox, InboxMember };
