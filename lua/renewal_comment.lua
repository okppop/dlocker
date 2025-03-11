-- This file intend to make comment without adding
-- unfuntional thing to embed file

-- return value from redis by condition
--[[
key is not set      -> nil (redis.nil)
value weren't match -> currentValue (string)
set expire fail     -> 0 (int64)
set expire success  -> 1 (int64)
]]