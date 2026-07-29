local paths = {"/plaintext-handler", "/json-static", "/json-serialize", "/"}
local i = 0
request = function()
  i = i + 1
  return wrk.format(nil, paths[(i % #paths) + 1])
end
