-- Bounded id space: a real param route, not a flood. The advisor's 1024-route
-- cap never fills, so variant B pays its full per-request cost rather than
-- short-circuiting on a nil entry.
local i = 0
request = function()
  i = i + 1
  return wrk.format(nil, "/users/" .. (i % 200))
end
