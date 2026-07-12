# Dashboard Session Lifetime Design

## Goal

Allow an administrator to configure how long **new dashboard login sessions** remain valid. The deployment is a personal intranet NewAPI instance, so the setting must support a long-lived login option while retaining predictable and secure cookie behavior.

## Confirmed scope

- Applies only to successful, complete dashboard logins.
- Takes effect only on the next successful login; existing browser sessions are not refreshed or re-signed.
- Covers password login, completed 2FA login, OAuth, Passkey, WeChat, and Telegram because they converge on `controller.setupLogin`.
- Does not change API keys, 2FA pending state, OAuth state, Passkey challenge state, secure-verification state, or logout behavior.
- No database migration is required: reuse the existing `options` table and Option API.

## Cookie semantics

`gin-contrib/sessions.Options.MaxAge` uses seconds:

- `0`: session cookie; expires when the browser closes.
- `< 0`: delete the cookie immediately.
- `> 0`: persistent cookie with that lifetime.

The product must not claim a browser cookie is literally permanent. The long-lived choice is ten years (3650 days); browsers or users can still remove site data.

## Selected approach

Store a system Option named `DashboardSessionLifetimeDays` and expose it in **System Settings → Authentication → Basic Authentication**.

- Default: `30`, preserving current behavior.
- Accepted range: `1..3650` days.
- UI presets: 7, 30, 90, 365, and 3650 days.
- The 3650-day preset is labelled **Long-lived (10 years)** with an explanation that browser data clearing can still end the session.
- The value is loaded/updated through the existing Option mechanism and read at successful login time, so no application restart is required.

## Backend design

1. Add an in-memory, validated accessor for `DashboardSessionLifetimeDays`.
   - Default to 30 when the option is absent.
   - Reject or safely fall back for invalid values; the admin API/UI will prevent values outside 1..3650.
2. Add `DashboardSessionOptions()` that returns a full `sessions.Options` value:
   - `Path: "/"`
   - configured positive `MaxAge`
   - preserve current `HttpOnly`, `Secure`, and `SameSite` behavior.
3. In `controller.setupLogin`, call `session.Options(DashboardSessionOptions())` immediately before `session.Save()`.
4. Leave the application-wide store options unchanged. This is intentional: temporary 2FA/OAuth/Passkey sessions continue using their current lifecycle rather than inheriting the long-lived dashboard setting.
5. Logout continues to clear/delete the session exactly as it does today.

## Frontend design

Extend the existing `BasicAuthSection` form and Auth settings defaults/types with a numeric dashboard-session lifetime field.

The form validates the range and uses the existing `useUpdateOption` flow. Its help text explicitly states that only future logins are affected and API keys/security-flow temporary sessions are excluded.

## Validation plan

### Backend

- Default absent option yields 30 days.
- Valid configured value yields the correct `MaxAge` in a successful dashboard login response.
- Password, OAuth, Passkey, WeChat, Telegram, and completed 2FA use the same `setupLogin` cookie options.
- 2FA pending-session writes do not call the dashboard session helper.
- Invalid configuration values are rejected/fall back safely.
- Logout still marks the session cookie for deletion.

### Frontend

- Authentication settings display the default 30-day value.
- Valid presets and custom values save through the existing Option API.
- Invalid values are blocked before submission.
- Help text accurately describes next-login-only behavior and long-lived-cookie limits.

### Environment rollout

1. Merge the feature independently into `release/test`.
2. Build/deploy 3011 and verify setting persistence plus `Set-Cookie` lifetime after a fresh login.
3. Merge the same feature independently into `release/prod`.
4. Build/deploy 3010 and repeat the production login verification.
5. Archive the feature independently into `main-local`, then clean up its task-owned worktree and branches.

## Non-goals

- Per-user “remember me” choices.
- Rolling session renewal on dashboard activity.
- Extending API key, OAuth state, Passkey challenge, 2FA pending, or secure-verification lifetimes.
- Claiming that a browser cookie can never be removed.
