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
import {
  act,
  cleanup,
  render,
  renderHook,
  screen,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { JdtokenApps } from '@/features/jdtoken-apps'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

let client: QueryClient
let status: Record<string, unknown>

beforeEach(async () => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  vi.stubEnv('VITE_HOME_VARIANT', 'jdtoken')
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en, zhCN: zh },
    interpolation: { escapeValue: false },
  })
  status = {
    system_name: 'JDOTOKEN',
    logo: '/jdtoken/mark-square-20261012.png',
    HeaderNavModules: { rankings: true },
  }
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  client.setQueryData(['status'], status)
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { success: true, data: url === '/api/status' ? status : '' },
  }))
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.unstubAllEnvs()
  localStorage.clear()
})

function Wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

async function mountApps() {
  const root = createRootRoute()
  const apps = createRoute({
    getParentRoute: () => root,
    path: '/agent-market',
    component: JdtokenApps,
  })
  const playground = createRoute({
    getParentRoute: () => root,
    path: '/playground',
    component: () => <h1>Playground destination</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([apps, playground]),
    history: createMemoryHistory({ initialEntries: ['/agent-market'] }),
  })
  await router.load()
  render(
    <Wrapper>
      <RouterProvider router={router} />
    </Wrapper>
  )
  return router
}

describe('JDOTOKEN applications', () => {
  it('places Apps directly before Rankings in the shared navigation', () => {
    const { result } = renderHook(useTopNavLinks, { wrapper: Wrapper })
    const links = result.current
    expect(
      links[links.findIndex((link) => link.href === '/rankings') - 1]
    ).toEqual({ title: 'Apps', href: '/agent-market' })
    expect(links.filter((link) => link.href === '/agent-market')).toHaveLength(
      1
    )
  })

  it('keeps Apps accessible when rankings are disabled', () => {
    status.HeaderNavModules = { rankings: false }
    client.setQueryData(['status'], status)
    const { result } = renderHook(useTopNavLinks, { wrapper: Wrapper })
    expect(result.current.some((link) => link.href === '/rankings')).toBe(false)
    expect(result.current.some((link) => link.href === '/agent-market')).toBe(
      true
    )
  })

  it('does not add the custom Apps entry to an upstream build', () => {
    vi.stubEnv('VITE_HOME_VARIANT', 'upstream')
    const { result } = renderHook(useTopNavLinks, { wrapper: Wrapper })
    expect(result.current.some((link) => link.href === '/agent-market')).toBe(
      false
    )
  })

  it('shows unavailable workspaces as disabled instead of offering broken launch links', async () => {
    await mountApps()
    expect(
      await screen.findByRole('heading', { name: 'Apps', level: 1 })
    ).toBeVisible()
    expect(screen.getAllByRole('article')).toHaveLength(6)
    for (const title of [
      'Image studio',
      'Video studio',
      'Prompt assistant',
      'Creative material library',
      'Agent workspace',
    ]) {
      const card = screen.getByRole('article', { name: title })
      expect(
        within(card).getByRole('button', { name: 'Coming soon' })
      ).toBeDisabled()
    }
    expect(
      screen.getByRole('link', { name: 'Video API guide' })
    ).toHaveAttribute('href', '/docs/#video-workflow')
    expect(
      screen.getByRole('link', { name: 'Material API guide' })
    ).toHaveAttribute('href', '/docs/#asset-mode')
    expect(document.querySelector('iframe')).toBeNull()
    expect(document.querySelector('a[href*="moyu.cn"]')).toBeNull()
  })

  it('opens the existing playground through keyboard navigation', async () => {
    const user = userEvent.setup()
    const router = await mountApps()
    const launch = await screen.findByRole('link', { name: 'Open playground' })
    launch.focus()
    await user.keyboard('{Enter}')
    expect(
      await screen.findByRole('heading', { name: 'Playground destination' })
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/playground')
  })

  it('updates card titles, descriptions and disabled labels when the language changes', async () => {
    await mountApps()
    expect(
      await screen.findByRole('article', { name: 'Image studio' })
    ).toBeVisible()
    await act(async () => {
      await i18n.changeLanguage('zhCN')
    })
    const card = screen.getByRole('article', { name: '图片生成' })
    expect(
      within(card).getByRole('button', { name: '即将上线' })
    ).toBeDisabled()
    expect(
      within(card).getByText(
        '将灵感和参考图片转化为视觉作品。图片创作工作台正在筹备，敬请期待。'
      )
    ).toBeVisible()
    expect(
      screen.getByRole('heading', { name: '应用', level: 1 })
    ).toBeVisible()
  })
})
