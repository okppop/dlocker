--[[
values weren't match -> 0
delete fail          -> 0
delete success       -> 1
]]

local key = KEYS[1]
local value = ARGV[1]
local currentValue = redis.call('GET', key)

if currentValue ~= value then
    return 0
end

return redis.call('DEL', key)