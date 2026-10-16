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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { Home } from '@/features/home'
import type { PricingModel } from '@/features/pricing/types'
import en from '@/i18n/locales/en.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

const model: PricingModel = {
  id: 1,
  model_name: 'provider/chat-model',
  description: 'A chat model from this gateway.',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 1,
  enable_groups: ['default'],
  supported_endpoint_types: ['openai'],
}
let client: QueryClient
let status: Record<string, unknown>
let models: PricingModel[]
let content: string
let failure: boolean
let pending: Promise<unknown> | undefined
let requested: string[]

beforeEach(async () => {
  localStorage.clear()
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  useAuthStore.getState().auth.reset()
  vi.stubEnv('VITE_HOME_VARIANT', 'jdtoken')
  status = {
    register_enabled: true,
    user_agreement_enabled: true,
    privacy_policy_enabled: true,
    server_address: 'https://gateway.example.com/proxy/',
    HeaderNavModules: JSON.stringify({
      pricing: { enabled: true, requireAuth: false },
      docs: false,
    }),
  }
  models = [model]
  content = ''
  failure = false
  pending = undefined
  requested = []
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en, zhCN: zh, zhTW },
    interpolation: { escapeValue: false },
  })
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    requested.push(url)
    if (url === '/api/status') return { data: { success: true, data: status } }
    if (url === '/api/pricing') {
      if (pending) await pending
      if (failure) {
        return { data: { success: false, message: 'Catalog unavailable' } }
      }
      return {
        data: {
          success: true,
          data: models,
          vendors: [],
          group_ratio: {},
          usable_group: {},
          supported_endpoint: {},
          auto_groups: [],
        },
      }
    }
    if (url === '/api/home_page_content') {
      return { data: { success: true, data: content } }
    }
    return { data: { success: true, data: '' } }
  })
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  localStorage.clear()
})

async function mountHome() {
  client.setQueryData(['status'], status)
  const root = createRootRoute()
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: Home,
  })
  const router = createRouter({
    routeTree: root.addChildren([home]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

describe('JDOTOKEN homepage', () => {
  it('hides the header version entry for an administrator with an unknown version', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('[]')))
    status.version = 'v0.0.0'
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'admin', role: 100, group: 'default' })
    await mountHome()
    expect(
      await screen.findByRole('heading', { name: model.model_name })
    ).toBeVisible()
    expect(
      screen.queryByRole('button', { name: /System updates, current version:/ })
    ).not.toBeInTheDocument()
  })

  it('shows gateway models and routes model details to the upstream catalog', async () => {
    await mountHome()
    expect(
      await screen.findByRole('heading', { name: model.model_name })
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: `View ${model.model_name} details` })
    ).toHaveAttribute('href', '/pricing/provider%2Fchat-model')
    expect(
      screen.getAllByRole('link', { name: 'Get started with JDOTOKEN' })[0]
    ).toHaveAttribute('href', '/sign-up')
    expect(
      screen.getByRole('link', { name: 'Try the playground' })
    ).toHaveAttribute('href', '/playground')
    expect(
      screen.getByRole('link', {
        name: 'Explore plans and manage your balance',
      })
    ).toHaveAttribute('href', '/wallet')
    expect(
      screen.getAllByRole('textbox', { name: 'quickstart.py' })[0].textContent
    ).toContain('https://gateway.example.com/proxy/v1')
    expect(
      screen.getAllByRole('textbox', { name: 'quickstart.py' })[0].textContent
    ).toContain('provider/chat-model')
    expect(
      screen.queryByRole('link', { name: 'New API' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'User Agreement' })
    ).toHaveAttribute('href', '/user-agreement')
    expect(
      screen.getByRole('link', { name: 'Privacy Policy' })
    ).toHaveAttribute('href', '/privacy-policy')
    expect(requested.every((url) => !url.includes('prod-api'))).toBe(true)
  })

  it('shows a loading state until model data arrives', async () => {
    let resolve: () => void = () => undefined
    pending = new Promise<void>((done) => {
      resolve = done
    })
    await mountHome()
    expect(await screen.findByText('Loading models...')).toBeVisible()
    await act(async () => resolve())
    expect(
      await screen.findByRole('heading', { name: model.model_name })
    ).toBeVisible()
  })

  it('shows an empty state without inventing models or prices', async () => {
    models = []
    await mountHome()
    expect(
      await screen.findByText('The model catalog is being prepared')
    ).toBeVisible()
    expect(
      screen.getAllByRole('textbox', { name: 'quickstart.py' })[0].textContent
    ).toContain('YOUR_MODEL_ID')
    expect(
      screen.queryByRole('link', { name: 'Explore all models' })
    ).not.toBeInTheDocument()
  })

  it('recovers from a failed business response using Retry', async () => {
    failure = true
    await mountHome()
    expect(
      await screen.findByText('Models are temporarily unavailable')
    ).toBeVisible()
    failure = false
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('heading', { name: model.model_name })
    ).toBeVisible()
  })

  it('does not request or expose the catalog when anonymous access requires login', async () => {
    status.HeaderNavModules = JSON.stringify({
      pricing: { enabled: true, requireAuth: true },
    })
    client.setQueryData(['jdtoken-home', 'catalog', null, null], {
      success: true,
      data: [model],
    })
    await mountHome()
    expect(
      await screen.findByText('Sign in to explore available models')
    ).toBeVisible()
    expect(requested).not.toContain('/api/pricing')
    expect(
      screen.queryByRole('heading', { name: model.model_name })
    ).not.toBeInTheDocument()
  })

  it('hides the model section when the catalog is disabled', async () => {
    status.HeaderNavModules = JSON.stringify({ pricing: false })
    await mountHome()
    await screen.findByRole('heading', {
      level: 1,
      name: 'Model access, made simple.',
    })
    expect(
      screen.queryByRole('heading', { name: 'Model Square' })
    ).not.toBeInTheDocument()
    expect(requested).not.toContain('/api/pricing')
  })

  it('uses sign-in instead of registration when registration is disabled', async () => {
    status.register_enabled = false
    await mountHome()
    await screen.findByRole('heading', { name: model.model_name })
    expect(
      screen.queryByRole('link', { name: 'Get started with JDOTOKEN' })
    ).not.toBeInTheDocument()
    expect(document.querySelector('main a[href="/sign-in"]')).not.toBeNull()
  })

  it('uses the dashboard for a signed-in user and clears restricted models on logout', async () => {
    status.HeaderNavModules = JSON.stringify({
      pricing: { enabled: true, requireAuth: true },
    })
    useAuthStore
      .getState()
      .auth.setUser({ id: 42, username: 'reader', role: 1, group: 'default' })
    await mountHome()
    expect(
      await screen.findByRole('heading', { name: model.model_name })
    ).toBeVisible()
    expect(document.querySelector('main a[href="/dashboard"]')).not.toBeNull()
    await act(async () => useAuthStore.getState().auth.reset())
    expect(
      await screen.findByText('Sign in to explore available models')
    ).toBeVisible()
    expect(
      screen.queryByRole('heading', { name: model.model_name })
    ).not.toBeInTheDocument()
  })

  it('updates homepage copy when switching between Chinese languages', async () => {
    await mountHome()
    await screen.findByRole('heading', {
      level: 1,
      name: 'Model access, made simple.',
    })
    await act(async () => {
      await i18n.changeLanguage('zhCN')
    })
    expect(
      screen.getByRole('heading', { level: 1, name: '模型调用，从未如此简单' })
    ).toBeVisible()
    await act(async () => {
      await i18n.changeLanguage('zhTW')
    })
    expect(
      screen.getByRole('heading', { level: 1, name: '模型呼叫，從未如此簡單' })
    ).toBeVisible()
  })

  it('preserves an administrator-defined custom homepage', async () => {
    content = '# Existing custom homepage'
    await mountHome()
    expect(
      await screen.findByRole('heading', { name: 'Existing custom homepage' })
    ).toBeVisible()
    expect(
      screen.queryByRole('heading', { name: 'Model access, made simple.' })
    ).not.toBeInTheDocument()
    expect(requested).not.toContain('/api/pricing')
  })

  it('retains the upstream homepage when the build switch is set to upstream', async () => {
    vi.stubEnv('VITE_HOME_VARIANT', 'upstream')
    await mountHome()
    await waitFor(() => expect(document.querySelector('h1')).not.toBeNull())
    expect(
      screen.queryByRole('heading', { name: 'Model access, made simple.' })
    ).not.toBeInTheDocument()
    expect(requested).not.toContain('/api/pricing')
  })
})
