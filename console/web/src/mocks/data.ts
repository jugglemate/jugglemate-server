import type { Inbox, User } from "@/api/schemas";
import { vercelAvatarUrl } from "./utils";

const MOCK_IDENTITIES: ReadonlyArray<[string, string]> = [
  ["admin", "ops.admin@northstar.io"],
  ["zhao.ming", "zhao.ming@northstar.io"],
  ["sarah.chen", "sarah.chen@northstar.io"],
  ["james.park", "james.park@northstar.io"],
  ["emma.garcia", "emma.garcia@northstar.io"],
  ["ryan.kim", "ryan.kim@northstar.io"],
  ["olivia.tanaka", "olivia.tanaka@northstar.io"],
  ["liam.patel", "liam.patel@northstar.io"],
  ["ava.nguyen", "ava.nguyen@northstar.io"],
  ["noah.berg", "noah.berg@northstar.io"],
  ["mia.silva", "mia.silva@northstar.io"],
  ["lucas.rossi", "lucas.rossi@northstar.io"],
  ["isabella.ali", "isabella.ali@northstar.io"],
  ["ethan.kovacs", "ethan.kovacs@northstar.io"],
  ["sophia.dubois", "sophia.dubois@northstar.io"],
  ["mateo.santos", "mateo.santos@northstar.io"],
  ["harper.ivanov", "harper.ivanov@northstar.io"],
  ["elijah.mohamed", "elijah.mohamed@northstar.io"],
  ["amelia.schmidt", "amelia.schmidt@northstar.io"],
  ["henry.okafor", "henry.okafor@northstar.io"],
];

export const GUEST_AUTH_USER_BODY = {
  id: "99",
  username: "guest",
  avatar: null as null,
  email: "guest@example.com",
  roles: [] as string[],
};

export const MOCK_USERS: User[] = MOCK_IDENTITIES.map(([username, email], i) => ({
  id: String(i + 1),
  username,
  avatar: vercelAvatarUrl(username),
  email,
  roles: i === 0 ? ["admin"] : ["editor"],
  permissions: i === 0 ? ["user:view", "user:create", "user:edit", "user:delete"] : ["user:view"],
}));

export const MOCK_INBOXES: Inbox[] = [
  {
    id: "inbox_telegram_1",
    name: "Telegram Support",
    channel_type: "telegram",
    channel_conf: {
      bot_name: "support_bot",
      bot_token: "",
    },
    member_count: 2,
    created_time: Date.now() - 86_400_000,
    updated_time: Date.now() - 3_600_000,
  },
  {
    id: "inbox_juggleim_1",
    name: "JuggleIM Support",
    channel_type: "juggleim",
    channel_conf: {
      bot_name: "juggle_support_bot",
      bot_token: "",
    },
    member_count: 1,
    created_time: Date.now() - 64_800_000,
    updated_time: Date.now() - 2_700_000,
  },
  {
    id: "inbox_widget_1",
    name: "Website Support",
    channel_type: "widget",
    channel_conf: {
      welcome_message: "Hi! How can we help you today?",
    },
    member_count: 1,
    created_time: Date.now() - 43_200_000,
    updated_time: Date.now() - 1_800_000,
  },
];

export const MOCK_INBOX_MEMBER_IDS: Record<string, string[]> = {
  inbox_telegram_1: ["1", "2"],
  inbox_juggleim_1: ["1"],
  inbox_widget_1: ["1"],
};
