import { createFileRoute } from "@tanstack/react-router";
import { ModelConfigView } from "./-ModelConfig";

export const Route = createFileRoute("/_auth/models/$providerId")({
  component: RouteComponent,
});

function RouteComponent() {
  const { providerId } = Route.useParams();
  return <ModelConfigView mode="edit" providerId={providerId} />;
}
