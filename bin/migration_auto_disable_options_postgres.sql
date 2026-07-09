-- Configure automatic channel disable rules for transient 429 rate limits.
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
