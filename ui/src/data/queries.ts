import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"

// Shared query hooks + helpers used across screens.

export function useServices() {
  return useQuery({ queryKey: ["services"], queryFn: api.listServices })
}

// Resolve a service_id → name for the many screens that reference services by id.
export function useServiceNames(): Record<string, string> {
  const { data } = useServices()
  return useMemo(() => {
    const m: Record<string, string> = {}
    for (const s of data ?? []) m[s.id] = s.name
    return m
  }, [data])
}
