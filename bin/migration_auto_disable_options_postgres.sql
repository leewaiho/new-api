-- Configure automatic channel disable rules for transient 429 rate limits.
-- Keep 429 in AutomaticDisableStatusCodes because some providers use 429 for
-- hard quota / usage-threshold failures that should disable a channel.
-- AutomaticDisableIgnoreKeywords only lists transient rate-limit messages:
-- request frequency is too high, retry shortly, or temporary throttling.
-- Do not add quota exhaustion / plan limit / credit balance messages here;
-- those belong in AutomaticDisableKeywords and should still auto-disable.
-- Keep these explanations as SQL comments, not option value lines, because
-- each value line is treated as a keyword by the application.
-- PostgreSQL version. Safe to run repeatedly.

INSERT INTO options (key, value)
VALUES
  ('AutomaticDisableStatusCodes', '401,429'),
  ('AutomaticDisableIgnoreKeywords', $kw$requests are too frequent
reduce your request frequency
wait a short moment
too many requests
rate limit
rate_limit
rate limited
request frequency
请求过于频繁
请求频率过高
请稍后重试
稍后再试$kw$)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value;
