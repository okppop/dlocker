local key = KEYS[1]
local value = ARGV[1]
local currentValue = redis.call('GET', key)
if currentValue ~= value then
    return currentValue
end
return redis.call('DEL', key)