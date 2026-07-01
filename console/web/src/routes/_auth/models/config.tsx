import { createFileRoute } from "@tanstack/react-router";
import { ModelConfigView } from "./-ModelConfig";

export const Route = createFileRoute("/_auth/models/config")({
  component: () => <ModelConfigView mode="create" />,
});
