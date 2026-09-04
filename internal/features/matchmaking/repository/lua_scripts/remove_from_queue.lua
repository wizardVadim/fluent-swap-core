local client_key = KEYS[1]
local client_id = ARGV[1]
local state = redis.call('HGET', client_key, 'state')
if state == false then
    return 0
end

if state == 'matched' then
    return 0
end

if state ~= 'waiting' then
    return -1
end

local queue_key = redis.call('HGET', client_key, 'queue_key')
if not queue_key then
    return -2
end
redis.call('LREM', queue_key, 0, client_id)
redis.call('DEL', client_key)

return 1