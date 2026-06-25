import { createFileRoute, redirect } from "@tanstack/react-router";
import { persistAppKeyFromUrl } from "@/utils/appkey";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    persistAppKeyFromUrl();
    throw redirect({ to: "/login" });
  },
});
