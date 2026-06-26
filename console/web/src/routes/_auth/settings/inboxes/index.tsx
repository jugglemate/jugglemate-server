import { createFileRoute, redirect } from "@tanstack/react-router";

/** Back-compat: old menu path before Inboxes moved out of Settings. */
export const Route = createFileRoute("/_auth/settings/inboxes/")({
  beforeLoad: () => {
    throw redirect({
      to: "/inboxes",
      search: { limit: 100, offset: 0 },
    });
  },
});
