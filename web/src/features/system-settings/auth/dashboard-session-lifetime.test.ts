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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  DASHBOARD_SESSION_LIFETIME_PRESETS,
  parseDashboardSessionLifetimeDaysInput,
} from './dashboard-session-lifetime'

describe('dashboard session lifetime input', () => {
  test('accepts the documented bounds', () => {
    assert.equal(parseDashboardSessionLifetimeDaysInput('1'), 1)
    assert.equal(parseDashboardSessionLifetimeDaysInput(30), 30)
    assert.equal(parseDashboardSessionLifetimeDaysInput('3650'), 3650)
  })

  test('rejects values outside the documented bounds and non-integers', () => {
    assert.equal(parseDashboardSessionLifetimeDaysInput('0'), null)
    assert.equal(parseDashboardSessionLifetimeDaysInput('-1'), null)
    assert.equal(parseDashboardSessionLifetimeDaysInput('3651'), null)
    assert.equal(parseDashboardSessionLifetimeDaysInput('90.5'), null)
    assert.equal(parseDashboardSessionLifetimeDaysInput('forever'), null)
  })

  test('keeps the administrator presets stable', () => {
    assert.deepEqual(DASHBOARD_SESSION_LIFETIME_PRESETS, [7, 30, 90, 365, 3650])
  })
})
