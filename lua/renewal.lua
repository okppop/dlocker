local key = KEYS[1]
local value = ARGV[1]
local currentValue = redis.call('GET', key)
local expireSeconds = ARGV[2]
if currentValue ~= value then
    return 0
end
return redis.call('EXPIRE', key, expireSeconds)