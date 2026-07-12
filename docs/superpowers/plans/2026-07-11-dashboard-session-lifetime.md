# Dashboard Session Lifetime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a root administrator configure the lifetime of newly issued dashboard login cookies, including a long-lived 10-year option, without changing API keys or temporary security-flow sessions.

**Architecture:** Persist `DashboardSessionLifetimeDays` in the existing Option system with a default of 30 days and a strict 1–3650 day range. Apply a complete `sessions.Options` value only from `controller.setupLogin`, which is the common successful-login convergence point; leave the store-level defaults untouched so temporary 2FA/OAuth/Passkey sessions retain their current lifecycle.

**Tech Stack:** Go, Gin, `gin-contrib/sessions` cookie store, GORM Option persistence, React, React Hook Form, Zod, existing System Settings Option API.

---

## File structure

| File | Responsibility |
|---|---|
| `setting/auth_setting/session_lifetime.go` | Constants, validation, default/fallback accessor, and full dashboard cookie options. |
| `setting/auth_setting/session_lifetime_test.go` | Pure unit tests for range validation and cookie option construction. |
| `common/constants.go` | Add the default in-memory dashboard-session lifetime setting. |
| `model/option.go` | Seed the option map and apply validated runtime updates. |
| `controller/option.go` | Reject invalid administrator Option API values before database persistence. |
| `controller/user.go` | Apply dashboard-specific options only during completed login before saving the session. |
| `controller/user_session_test.go` | HTTP-level cookie tests for configured dashboard session issuance and temporary-session isolation. |
| `web/default/src/features/system-settings/types.ts` | Add the authentication settings field type. |
| `web/default/src/features/system-settings/auth/index.tsx` | Add the default value returned to the Authentication settings page. |
| `web/default/src/features/system-settings/auth/section-registry.tsx` | Pass the setting to Basic Authentication. |
| `web/default/src/features/system-settings/auth/basic-auth-section.tsx` | Add a validated number input, presets, help text, and Option update. |

## Task 1: Add a validated session-lifetime domain helper

**Files:**
- Create: `setting/auth_setting/session_lifetime.go`
- Test: `setting/auth_setting/session_lifetime_test.go`

- [ ] **Step 1: Write failing helper tests**

```go
func TestParseDashboardSessionLifetimeDays(t *testing.T) {
    cases := []struct {
        raw  string
        want int
        ok   bool
    }{
        {"1", 1, true},
        {"30", 30, true},
        {"3650", 3650, true},
        {"0", 0, false},
        {"3651", 0, false},
        {"-1", 0, false},
        {"not-a-number", 0, false},
    }
    for _, tt := range cases {
        got, err := ParseDashboardSessionLifetimeDays(tt.raw)
        if tt.ok && (err != nil || got != tt.want) { t.Fatalf("ParseDashboardSessionLifetimeDays(%q) = (%d, %v), want (%d, nil)", tt.raw, got, err, tt.want) }
        if !tt.ok && err == nil { t.Fatalf("ParseDashboardSessionLifetimeDays(%q) unexpectedly succeeded with %d", tt.raw, got) }
    }
}

func TestDashboardSessionOptionsPreservesCookieSecurityAttributes(t *testing.T) {
    options := DashboardSessionOptions(90)
    if options.MaxAge != 90*24*60*60 { t.Fatalf("MaxAge = %d, want %d", options.MaxAge, 90*24*60*60) }
    if options.Path != "/" || !options.HttpOnly || options.Secure || options.SameSite != http.SameSiteStrictMode { t.Fatalf("dashboard cookie security options changed: %+v", options) }
}
```

- [ ] **Step 2: Run the test to verify red state**

Run:

```bash
docker run --rm -e GODEBUG=http2client=0 -v "$PWD:/src" -w /src -v newapi-go-modcache:/go/pkg/mod golang:1.25.1 go test ./setting/auth_setting -count=1
```

Expected: package/function compilation failure because `auth_setting` and its helpers do not exist.

- [ ] **Step 3: Implement the helper**

Create a package with these public constants/functions:

```go
const (
    OptionDashboardSessionLifetimeDays = "DashboardSessionLifetimeDays"
    DefaultDashboardSessionLifetimeDays = 30
    MaxDashboardSessionLifetimeDays = 3650
)

func ParseDashboardSessionLifetimeDays(raw string) (int, error)
func DashboardSessionOptions(days int) sessions.Options
```

`ParseDashboardSessionLifetimeDays` must parse base-10 integers and reject values outside `1..3650`. `DashboardSessionOptions` must build the complete options struct rather than setting only `MaxAge`, preserving the current `Path`, `HttpOnly`, `Secure`, and `SameSite` values.

- [ ] **Step 4: Run the helper test to verify green state**

Run the same command from Step 2.

Expected: `ok github.com/QuantumNous/new-api/setting/auth_setting`.

- [ ] **Step 5: Commit the isolated helper**

```bash
git add setting/auth_setting/session_lifetime.go setting/auth_setting/session_lifetime_test.go
git commit -m "feat: define dashboard session lifetime options"
```

## Task 2: Persist and validate the system Option

**Files:**
- Modify: `common/constants.go`
- Modify: `model/option.go`
- Modify: `controller/option.go`
- Test: `setting/auth_setting/session_lifetime_test.go`

- [ ] **Step 1: Add failing runtime-option tests**

Add tests that prove:

```go
// absent Option value resolves to DefaultDashboardSessionLifetimeDays
// an update to "90" changes the runtime lifetime to 90
// "0", "-1", "3651", and non-numeric input are rejected before UpdateOption persists them
```

Keep the tests independent of a real production database by using the existing in-memory OptionMap/SQLite test pattern.

- [ ] **Step 2: Run only the new option tests**

Run:

```bash
docker run --rm -e GODEBUG=http2client=0 -v "$PWD:/src" -w /src -v newapi-go-modcache:/go/pkg/mod golang:1.25.1 go test ./setting/auth_setting ./model ./controller -run 'Test.*DashboardSessionLifetime' -count=1
```

Expected: failure because the Option is neither seeded nor validated.

- [ ] **Step 3: Implement Option plumbing**

1. Add `DashboardSessionLifetimeDays = 30` to `common/constants.go`.
2. Seed `DashboardSessionLifetimeDays` in `model.InitOptionMap` using the default value.
3. In `model.updateOptionMap`, add a dedicated case that parses through `auth_setting.ParseDashboardSessionLifetimeDays` and updates `common.DashboardSessionLifetimeDays` only after validation.
4. In `controller.UpdateOption`, add an early case for `auth_setting.OptionDashboardSessionLifetimeDays`. Convert the incoming value to string, validate it, and return HTTP 400 with `success:false` when invalid. Call `model.UpdateOption` only for valid values.
5. Ensure an old or malformed database value falls back to 30 at startup rather than preventing application boot.

- [ ] **Step 4: Run the option tests to verify green state**

Run the command from Step 2.

Expected: all dashboard-lifetime tests pass.

- [ ] **Step 5: Commit Option plumbing**

```bash
git add common/constants.go model/option.go controller/option.go setting/auth_setting/session_lifetime.go setting/auth_setting/session_lifetime_test.go
git commit -m "feat: persist dashboard session lifetime setting"
```

## Task 3: Apply the lifetime only to completed dashboard logins

**Files:**
- Modify: `controller/user.go`
- Create: `controller/user_session_test.go`

- [ ] **Step 1: Write failing cookie issuance tests**

Create an HTTP test router using `cookie.NewStore`, `sessions.Sessions("session", store)`, and a handler that calls the same helper used by `setupLogin`.

Test these assertions:

```go
func TestCompletedDashboardLoginUsesConfiguredLifetime(t *testing.T) {
    // configure 90 days
    // perform a successful session save through the completed-login helper
    // assert the Set-Cookie header has Max-Age == 90*24*60*60
    // assert HttpOnly and SameSite=Strict remain present
}

func TestTemporarySessionDoesNotUseDashboardLifetime(t *testing.T) {
    // configure 3650 days
    // save a pending_2fa-style session without the completed-login helper
    // assert it does not receive the 3650-day Max-Age override
}
```

- [ ] **Step 2: Run cookie tests to verify red state**

Run:

```bash
docker run --rm -e GODEBUG=http2client=0 -v "$PWD:/src" -w /src -v newapi-go-modcache:/go/pkg/mod golang:1.25.1 go test ./controller -run 'Test.*Dashboard.*Session|TestTemporarySession' -count=1
```

Expected: the configured lifetime is not present because `setupLogin` still uses store defaults.

- [ ] **Step 3: Implement the completed-login override**

In `controller/user.go`, add a narrow helper:

```go
func applyDashboardSessionOptions(session sessions.Session) {
    session.Options(auth_setting.DashboardSessionOptions(common.DashboardSessionLifetimeDays))
}
```

Call it in `setupLogin` after setting `id`, `username`, `role`, `status`, and `group`, immediately before `session.Save()`.

Do not call it in the password-login 2FA pending branch, `Logout`, Passkey challenge helpers, OAuth state helpers, or secure-verification helpers. The existing Password/OAuth/Passkey/WeChat/Telegram/completed-2FA paths already converge on `setupLogin`, so no duplicate per-provider changes are needed.

- [ ] **Step 4: Run cookie tests to verify green state**

Run the command from Step 2.

Expected: configured complete-login cookie passes; temporary session remains unmodified.

- [ ] **Step 5: Commit completed-login behavior**

```bash
git add controller/user.go controller/user_session_test.go
git commit -m "feat: apply configured dashboard session lifetime on login"
```

## Task 4: Add the administrator UI

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/auth/index.tsx`
- Modify: `web/default/src/features/system-settings/auth/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/auth/basic-auth-section.tsx`

- [ ] **Step 1: Extend the form schema and defaults**

Add `DashboardSessionLifetimeDays: z.coerce.number().int().min(1).max(3650)` to `basicAuthSchema`, include it in `BasicAuthFormValues`, Auth settings type/defaults, and section-registry props.

- [ ] **Step 2: Add a failing frontend typecheck expectation**

Run:

```bash
cd web && bun install --frozen-lockfile && bun run typecheck
```

Expected before the implementation: TypeScript errors for the missing AuthSettings field/schema wiring.

- [ ] **Step 3: Implement the form control**

Use a numeric form field in Basic Authentication with preset buttons for `7`, `30`, `90`, `365`, and `3650` days. The label/help text must say:

```text
Dashboard login session lifetime
Only applies to future successful dashboard logins. API keys and temporary OAuth, Passkey, and 2FA sessions are unchanged.
```

For 3650 days, show:

```text
Long-lived (10 years). Browsers can still remove site data or cookies.
```

On submit, include `DashboardSessionLifetimeDays` in the same `useUpdateOption` update loop only when it differs from the original value.

- [ ] **Step 4: Run frontend checks**

Run in an isolated temporary container copy so `node_modules` and lockfiles are not written to the worktree:

```bash
docker run --rm -v "$PWD:/src:ro" oven/bun:1 sh -lc 'cp -a /src /work && cd /work/web && bun install --frozen-lockfile && bun run typecheck'
```

Then run the repository formatter/linter command for only the modified files.

Expected: typecheck passes and touched files have zero formatter/linter errors.

- [ ] **Step 5: Commit the UI**

```bash
git add web/default/src/features/system-settings/types.ts web/default/src/features/system-settings/auth/index.tsx web/default/src/features/system-settings/auth/section-registry.tsx web/default/src/features/system-settings/auth/basic-auth-section.tsx
git commit -m "feat: configure dashboard session lifetime"
```

## Task 5: Full feature verification and test-environment promotion

**Files:**
- Verify only; no planned code change.

- [ ] **Step 1: Run the complete targeted backend suite**

```bash
docker run --rm -e GODEBUG=http2client=0 -v "$PWD:/src" -w /src -v newapi-go-modcache:/go/pkg/mod golang:1.25.1 go test ./controller ./model ./setting/auth_setting ./relay ./dto ./relay/channel/advancedcustom ./service/relayconvert -count=1
```

Expected: every named package passes.

- [ ] **Step 2: Review the feature diff**

```bash
git diff --check 156f0f17..HEAD
git status --short
git log --oneline 156f0f17..HEAD
```

Expected: no whitespace errors; only the design/plan plus session-lifetime files are changed.

- [ ] **Step 3: Request/complete independent merge to release/test**

From the `release-test` worktree:

```bash
git merge --no-ff feature/202607-dashboard-session-lifetime -m 'merge: dashboard session lifetime into release/test'
git push origin release/test
```

Do not merge `release/test` into any other branch.

- [ ] **Step 4: Wait for test GHCR CI and deploy 3011 app only**

After the test workflow succeeds:

```bash
ssh homelab 'cd /opt/newapi-test && docker compose pull new-api-test && docker compose up -d --no-deps --force-recreate new-api-test'
```

Verify `new-api-test` is `running / healthy`; do not recreate `new-api-test-pg` or `new-api-test-redis`.

- [ ] **Step 5: Perform 3011 dashboard acceptance**

1. Open System Settings → Authentication → Basic Authentication.
2. Set 3650 days and save.
3. Logout, then perform a new dashboard login.
4. Inspect the new `Set-Cookie` response/cookie expiry; confirm approximately ten years and `HttpOnly`/`SameSite=Strict` remain.
5. Change to 30 days, save, logout/login again, and confirm 30-day expiry.
6. Confirm an already-created cookie was not rewritten merely by changing the setting.

## Task 6: Production promotion and archive after test acceptance

**Files:**
- Verify/release only; no planned code change.

- [ ] **Step 1: Independently merge the feature to release/prod**

```bash
cd /home/weihao/projects/new-api-worktree/release-prod
git merge --no-ff feature/202607-dashboard-session-lifetime -m 'merge: dashboard session lifetime into release/prod'
git push origin release/prod
```

Do not merge `release/test` into `release/prod`.

- [ ] **Step 2: Wait for production GHCR CI and deploy 3010 app only**

```bash
ssh homelab 'cd /home/weihao/docker/newapi && docker compose pull new-api && docker compose up -d --no-deps --force-recreate new-api'
```

Verify `newapi-new-api-1` is `running / healthy`; preserve `newapi-postgres-1` and `newapi-redis-1`.

- [ ] **Step 3: Perform 3010 production acceptance**

Repeat the 3650-day fresh-login cookie check and verify `/api/status`. Scan app logs since deployment for `panic` or `fatal` before calling production healthy.

- [ ] **Step 4: Independently archive to main-local**

```bash
cd /home/weihao/projects/new-api-worktree/main-local
git merge --no-ff feature/202607-dashboard-session-lifetime -m 'merge: dashboard session lifetime into main-local'
git push origin main-local
git merge-base --is-ancestor feature/202607-dashboard-session-lifetime main-local
```

- [ ] **Step 5: Remove only the task-owned branch and worktree after ancestry verification**

```bash
cd /home/weihao/projects/new-api
git worktree remove /home/weihao/projects/new-api-worktree/feature-202607-dashboard-session-lifetime
git worktree prune
git branch -d feature/202607-dashboard-session-lifetime
git push origin --delete feature/202607-dashboard-session-lifetime
```

Do not remove `origin/pr/smart-api-type-routing` or modify the main worktree’s user-owned `.gitignore` change.
