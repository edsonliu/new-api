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
import { createInstance } from 'i18next'
import { describe, expect, test } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import { parseTiersFromExpr } from '../lib/billing-expr'
import { formatBillingCondition } from '../lib/billing-expression/condition-display'
import { parseBillingDate } from '../lib/billing-expression/date'
import { compileBillingExpression } from '../lib/billing-expression/parser'
import { evaluateBillingExpression } from '../lib/billing-expression/runtime'
import { parseVisualBillingDocument } from '../lib/billing-expression/visual'
import { getDynamicPricingSummary } from '../lib/dynamic-price'
import type { PricingModel } from '../types'
import fixtures from './fixtures/deepseek-calendar.json'

const holidays = [
  ['2026-01-01', '2026-01-04'],
  ['2026-02-15', '2026-02-24'],
  ['2026-04-04', '2026-04-07'],
  ['2026-05-01', '2026-05-06'],
  ['2026-06-19', '2026-06-22'],
  ['2026-09-25', '2026-09-28'],
  ['2026-10-01', '2026-10-08'],
]
function model(fixture: (typeof fixtures)[number]): PricingModel {
  return {
    ...fixture,
    id: 1,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    billing_mode: 'tiered_expr',
  }
}
describe('calendar-aware DeepSeek price display', () => {
  test.each(fixtures)(
    'extracts both complete tiers for $model_name',
    (fixture) => {
      const tiers = parseTiersFromExpr(fixture.billing_expr)
      expect(tiers.map((tier) => tier.label)).toEqual(['peak', 'off_peak'])
      expect(
        tiers.every((tier) => tier.conditionText?.includes('2026-10-08'))
      ).toBe(true)
      expect(parseVisualBillingDocument(fixture.billing_expr)).toBeNull()
      const prices = fixture.model_name.includes('pro')
        ? [9, 27, 0.3]
        : [2, 8, 0.04]
      for (const [now, peak] of [
        ['2026-10-16T09:00:00+08:00', true],
        ['2026-10-16T12:00:00+08:00', false],
        ['2026-10-16T14:00:00+08:00', true],
        ['2026-10-16T18:00:00+08:00', false],
        ['2026-10-01T09:00:00+08:00', false],
        ['2026-10-07T10:00:00+08:00', false],
        ['2026-10-08T09:00:00+08:00', true],
        ['2026-10-17T09:00:00+08:00', false],
      ] as const) {
        const summary = getDynamicPricingSummary(model(fixture), {
          tokenUnit: 'M',
          now: new Date(now),
        })
        expect(summary?.isSpecialExpression).toBe(false)
        expect(summary?.isTimePricing).toBe(true)
        expect(summary?.tier?.label).toBe(peak ? 'peak' : 'off_peak')
        expect(summary?.tiers).toHaveLength(2)
        const entries = [
          ...(summary?.primaryEntries ?? []),
          ...(summary?.secondaryEntries ?? []),
        ]
        for (const [index, field] of [
          'inputPrice',
          'outputPrice',
          'cacheReadPrice',
        ].entries()) {
          expect(
            entries.find((entry) => entry.field === field)?.value
          ).toBeCloseTo((prices[index] / 6.69) * (peak ? 1 : 0.5), 10)
        }
      }
    }
  )
  test('matches the 2026 calendar at every peak boundary without losing holidays', () => {
    for (let day = 0; day < 365; day++) {
      const date = new Date(Date.UTC(2026, 0, 1 + day))
      const iso = date.toISOString().slice(0, 10)
      const working =
        date.getUTCDay() >= 1 &&
        date.getUTCDay() <= 5 &&
        !holidays.some(([start, end]) => iso >= start && iso < end)
      for (const hour of [0, 8, 9, 11, 12, 13, 14, 17, 18, 23]) {
        const now = new Date(
          `${iso}T${String(hour).padStart(2, '0')}:00:00+08:00`
        )
        const peak =
          working && ((hour >= 9 && hour < 12) || (hour >= 14 && hour < 18))
        const result = evaluateBillingExpression(fixtures[2].billing_expr, {
          now,
          tokens: { p: 1000, c: 100, cr: 50 },
        })
        expect(result.status, now.toISOString()).toBe('success')
        if (result.status !== 'success') {
          throw new Error(result.diagnostic.detail)
        }
        expect(result.matchedTier, now.toISOString()).toBe(
          peak ? 'peak' : 'off_peak'
        )
        expect(result.cost).toBeCloseTo(
          ((1000 * 2 + 100 * 8 + 50 * 0.04) / 6.69) * (peak ? 1 : 0.5),
          7
        )
      }
    }
  })
  test('describes holiday conditions as text and preserves offsets', async () => {
    const translations = createInstance()
    await translations.init({ lng: 'zh', resources: { zh } })
    const condition =
      parseTiersFromExpr(fixtures[0].billing_expr)[0].conditionText ?? ''
    const text = formatBillingCondition(condition, translations.t, 'zhCN')
    expect(text).toContain('当前时间')
    expect(text).toContain('2026-10-01 00:00:00+08:00')
    expect(text).toContain('Asia/Shanghai')
    expect(text).not.toMatch(/now\(|date\(|weekday\(|hour\(/)
  })
  test.each(['<', '<=', '>', '>=', '==', '!='])(
    'keeps timestamp comparison %s typed',
    (op) => {
      const condition = `now() ${op} date("2026-10-01T00:00:00+08:00")`
      for (const delta of [-1, 0, 1]) {
        const now = new Date(Date.parse('2026-10-01T00:00:00+08:00') + delta)
        const matches: Record<string, boolean> = {
          '<': delta < 0,
          '<=': delta <= 0,
          '>': delta > 0,
          '>=': delta >= 0,
          '==': delta === 0,
          '!=': delta !== 0,
        }
        expect(
          evaluateBillingExpression(`${condition} ? 2 : 1`, { now })
        ).toMatchObject({ status: 'success', cost: matches[op] ? 2 : 1 })
      }
    }
  )
  test('compares equivalent instants rather than date object identity', () => {
    expect(
      evaluateBillingExpression(
        'date("2026-10-01T00:00:00+08:00") == date("2026-09-30T16:00:00Z") ? 2 : 1'
      )
    ).toMatchObject({ status: 'success', cost: 2 })
    expect(parseBillingDate('2024-02-29').toISOString()).toBe(
      '2024-02-29T00:00:00.000Z'
    )
    expect(
      parseBillingDate('2026-10-01T00:00:00.123+08:00').getUTCMilliseconds()
    ).toBe(123)
  })
  test.each([
    '2026-02-29',
    '2026-02-30',
    '2026-13-01',
    '2026-10-01T24:00:00Z',
    '2026-10-01T00:00:00',
    '2026-10-01T00:00:00+24:00',
    '2026-10-01T00:00:00.123456Z',
  ])('rejects ambiguous or unsupported dates: %s', (date) => {
    expect(
      compileBillingExpression(
        `now() < date("${date}") ? tier("a", p * 1) : tier("b", p * 2)`
      ).status
    ).not.toBe('ready')
  })
  test.each([
    'now() < 42',
    'date(42)',
    'now(1)',
    'now() + 1',
    'date("2026-10-01", "2006-01-02")',
  ])('does not reinterpret unsupported datetime expressions: %s', (source) => {
    expect(compileBillingExpression(source).status).not.toBe('ready')
  })
})
