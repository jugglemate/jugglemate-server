import { authHandlers } from "./auth";
import { inboxHandlers } from "./inbox";
import { userHandlers } from "./user";

export const handlers = [...authHandlers, ...userHandlers, ...inboxHandlers];
