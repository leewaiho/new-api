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
export const DASHBOARD_SESSION_LIFETIME_MIN_DAYS = 1
export const DASHBOARD_SESSION_LIFETIME_MAX_DAYS = 3650
export const DASHBOARD_SESSION_LIFETIME_PRESETS = [
  7, 30, 90, 365, 3650,
] as const

export function parseDashboardSessionLifetimeDaysInput(
  value: unknown
): number | null {
  let normalized = Number.NaN
  if (typeof value === 'number') {
    normalized = value
  } else if (typeof value === 'string' && value.trim() !== '') {
    normalized = Number(value)
  }

  if (
    !Number.isInteger(normalized) ||
    normalized < DASHBOARD_SESSION_LIFETIME_MIN_DAYS ||
    normalized > DASHBOARD_SESSION_LIFETIME_MAX_DAYS
  ) {
    return null
  }

  return normalized
}
