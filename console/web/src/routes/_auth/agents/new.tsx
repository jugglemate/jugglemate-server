import { createFileRoute } from "@tanstack/react-router";
import { AgentConfigView } from "./-AgentConfig";

export const Route = createFileRoute("/_auth/agents/new")({
  component: () => <AgentConfigView mode="create" />,
});
