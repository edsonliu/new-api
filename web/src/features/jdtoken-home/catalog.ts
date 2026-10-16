/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { getPricing } from '@/features/pricing/api'
import { useStatus } from '@/hooks/use-status'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

/** The only model API dependency of the branded homepage. No billing math here. */
export function useHomeCatalog() {
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const { status, loading: statusLoading, error: statusError } = useStatus()
  const access = parseHeaderNavModulesFromStatus(status).pricing
  const needsLogin = access.requireAuth && !user
  const allowed = !!status && !statusError && access.enabled && !needsLogin
  const query = useQuery({
    queryKey: [
      'jdtoken-home',
      'catalog',
      user?.id ?? null,
      user?.group ?? null,
    ],
    queryFn: async () => requireServerSuccess(await getPricing()),
    enabled: allowed && !statusLoading,
    staleTime: 60_000,
    retry: false,
    meta: { errorToast: false },
  })

  // Do not display cached private catalog data after access is withdrawn.
  const models = allowed ? (query.data?.data ?? []) : []
  const featured = models
    .filter((model) => model.model_name?.trim())
    .slice(0, 8)
  const chatModel = models.find((model) =>
    model.supported_endpoint_types?.includes('openai')
  )

  return {
    featured,
    chatModel: chatModel?.model_name,
    disabled: !!status && !access.enabled,
    needsLogin,
    loading: statusLoading || (allowed && query.isLoading),
    error: statusError ?? query.error,
    refetch: async () => {
      if (statusError) {
        await queryClient.refetchQueries({ queryKey: ['status'], exact: true })
      } else {
        await query.refetch()
      }
    },
  }
}
