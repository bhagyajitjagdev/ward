import { createFileRoute, Outlet } from "@tanstack/react-router"

// Pass-through layout so /services (list) and /services/$id (detail) render standalone.
export const Route = createFileRoute("/_app/services")({
  component: () => <Outlet />,
})
