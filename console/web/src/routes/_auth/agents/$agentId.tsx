import { createFileRoute } from "@tanstack/react-router";
import { AgentConfigView } from "./-AgentConfig";

export const Route = createFileRoute("/_auth/agents/$agentId")({
  component: RouteComponent,
});

function RouteComponent() {
  const { agentId } = Route.useParams();
  return <AgentConfigView mode="edit" agentId={agentId} />;
}
