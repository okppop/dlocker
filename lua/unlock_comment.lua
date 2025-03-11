-- This file intend to make comment without adding
-- unfuntional thing to embed file

-- return value from redis by condition
--[[
key is not set       -> nil (redis.nil)
values weren't match -> currentValue (string)
delete fail          -> 0 (int64)
delete success       -> 1 (int64)
]]