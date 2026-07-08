import { useQuery, useQueryClient } from "@tanstack/react-query"
import { api, getToken, setToken } from "./api"

// Current authenticated user (only fetched when a token is present).
export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    enabled: !!getToken(),
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return async () => {
    try {
      await api.logout()
    } catch {
      // ignore — clearing the token locally is enough
    }
    setToken(null)
    qc.clear()
  }
}
