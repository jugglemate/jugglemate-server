import type { User } from "@/api/schemas";
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
