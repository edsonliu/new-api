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
import { BillingExpressionError } from './types'

/** Explicit-offset RFC3339 and UTC dates only; never use browser-local parsing. */
export function parseBillingDate(source: string): Date {
  const match = source.match(
    /^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,3}))?(Z|[+-]\d{2}:\d{2}))?$/
  )
  if (!match) {
    throw new BillingExpressionError({
      code: 'unsupported',
      detail:
        'date requires a UTC date or RFC3339 timestamp with millisecond precision',
    })
  }
  const [, year, month, day, hour, minute, second, , offset] = match
  const y = Number(year)
  const m = Number(month)
  const d = Number(day)
  const leap = y % 4 === 0 && (y % 100 !== 0 || y % 400 === 0)
  const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
  if (
    m < 1 ||
    m > 12 ||
    d < 1 ||
    d > days[m - 1] ||
    (hour !== undefined &&
      (Number(hour) > 23 || Number(minute) > 59 || Number(second) > 59)) ||
    (offset &&
      offset !== 'Z' &&
      (Number(offset.slice(1, 3)) > 23 || Number(offset.slice(4)) > 59))
  ) {
    throw new BillingExpressionError({ code: 'type', detail: 'invalid date' })
  }
  const value = new Date(offset ? source : `${source}T00:00:00Z`)
  if (!Number.isFinite(value.getTime())) {
    throw new BillingExpressionError({ code: 'type', detail: 'invalid date' })
  }
  return value
}
