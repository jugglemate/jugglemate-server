import { http } from "msw";
import { InboxMembersResponseSchema, InboxSchema } from "@/api/schemas";
import { MOCK_INBOX_MEMBER_IDS, MOCK_INBOXES, MOCK_USERS } from "../data";
import { paginateList, parsePaginationParams } from "../utils";
import {
  paginatedWithSchema,
  successWithNullBody,
  successWithSchema,
  withDelay,
} from "../createHandler";

let inboxes = [...MOCK_INBOXES];
let inboxMemberIds: Record<string, string[]> = { ...MOCK_INBOX_MEMBER_IDS };

function updateMemberCount(inboxId: string) {
  const inbox = inboxes.find((item) => item.id === inboxId);
  if (inbox) inbox.member_count = inboxMemberIds[inboxId]?.length ?? 0;
}

function stringField(body: Record<string, unknown>, key: string) {
  const value = body[key];
  return typeof value === "string" ? value : "";
}

export const inboxHandlers = [
  http.get("/jmate/console/inboxes", async ({ request }) => {
    await withDelay(200);
    const url = new URL(request.url);
    const { limit, offset } = parsePaginationParams(url.searchParams);
    const list = paginateList(inboxes, limit, offset);
    return paginatedWithSchema(InboxSchema, { list, total: inboxes.length });
  }),

  http.post("/jmate/console/inboxes/telegram", async ({ request }) => {
    await withDelay(200);
    const body = (await request.json()) as Record<string, unknown>;
    const id = `inbox_${Date.now()}`;
    const inbox = {
      id,
      name: stringField(body, "name"),
      channel_type: "telegram" as const,
      channel_conf: {
        bot_name: stringField(body, "bot_name"),
        bot_token: "",
      },
      member_count: 0,
      created_time: Date.now(),
      updated_time: Date.now(),
    };
    inboxes = [inbox, ...inboxes];
    inboxMemberIds[id] = [];
    return successWithSchema(InboxSchema, inbox);
  }),

  http.post("/jmate/console/inboxes/juggleim", async ({ request }) => {
    await withDelay(200);
    const body = (await request.json()) as Record<string, unknown>;
    const id = `inbox_juggleim_${Date.now()}`;
    const inbox = {
      id,
      name: stringField(body, "name"),
      channel_type: "juggleim" as const,
      channel_conf: {
        bot_name: stringField(body, "bot_name"),
        bot_token: "",
      },
      member_count: 0,
      created_time: Date.now(),
      updated_time: Date.now(),
    };
    inboxes = [inbox, ...inboxes];
    inboxMemberIds[id] = [];
    return successWithSchema(InboxSchema, inbox);
  }),

  http.post("/jmate/console/inboxes/whatsapp", async ({ request }) => {
    await withDelay(200);
    const body = (await request.json()) as Record<string, unknown>;
    const id = `inbox_whatsapp_${Date.now()}`;
    const inbox = {
      id,
      name: stringField(body, "name"),
      channel_type: "whatsapp" as const,
      channel_conf: {
        phone_number_id: stringField(body, "phone_number_id"),
        display_phone_number: "+15551234567",
        verified_name: "Demo WhatsApp",
        api_version: "v24.0",
      },
      member_count: 0,
      created_time: Date.now(),
      updated_time: Date.now(),
    };
    inboxes = [inbox, ...inboxes];
    inboxMemberIds[id] = [];
    return successWithSchema(InboxSchema, inbox);
  }),

  http.post("/jmate/console/inboxes/widget", async ({ request }) => {
    await withDelay(200);
    const body = (await request.json()) as Record<string, unknown>;
    const id = `inbox_widget_${Date.now()}`;
    const inbox = {
      id,
      name: stringField(body, "name"),
      channel_type: "widget" as const,
      channel_conf: {
        welcome_message: stringField(body, "welcome_message"),
      },
      member_count: 0,
      created_time: Date.now(),
      updated_time: Date.now(),
    };
    inboxes = [inbox, ...inboxes];
    inboxMemberIds[id] = [];
    return successWithSchema(InboxSchema, inbox);
  }),

  http.get("/jmate/console/inboxes/:id/members", async ({ params }) => {
    await withDelay(200);
    const ids = inboxMemberIds[String(params.id)] ?? [];
    const list = MOCK_USERS.filter((user) => ids.includes(user.id)).map((user) => ({
      id: user.id,
      username: user.username,
      avatar: user.avatar,
      email: user.email,
    }));
    return successWithSchema(InboxMembersResponseSchema, { list });
  }),

  http.put("/jmate/console/inboxes/:id/members", async ({ params, request }) => {
    await withDelay(200);
    const id = String(params.id);
    const body = (await request.json()) as Record<string, unknown>;
    inboxMemberIds[id] = Array.isArray(body.user_ids) ? (body.user_ids as string[]) : [];
    updateMemberCount(id);
    return successWithNullBody();
  }),
];
