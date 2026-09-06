local client_key = KEYS[1]
local partner_queue_key = KEYS[2]
local client_queue_key = KEYS[3]

local exists = redis.call('EXISTS', client_key)
if exists == 1 then
    local state = redis.call('HGET', client_key, 'state')
    if state == false or state == '' then
        return {-2}
    end
    -- client is matched already
    if state == 'matched' then
        local partner_client_id = redis.call('HGET', client_key, 'partner_client_id')
        if partner_client_id == false or partner_client_id == '' then
            return {-2}
        end
        local stored_partner_queue_key = redis.call('HGET', client_key, 'partner_queue_key')
        if stored_partner_queue_key == false or stored_partner_queue_key == '' then
            return {-2}
        end
        return {1, partner_client_id, stored_partner_queue_key}
    end
    -- client is waiting already
    if state == "waiting" then
        local queue_key = redis.call('HGET', client_key, 'queue_key')
        if queue_key == false or queue_key == '' then
            return {-2}
        end
        if queue_key ~= client_queue_key then
            return {-1}
        end
        return {0}
    end
    return {-2}
end
-- find any partner or enqueue
local client_id = ARGV[1]
local waiting_ttl = tonumber(ARGV[2])
if waiting_ttl == nil or waiting_ttl <= 0 or waiting_ttl % 1 ~= 0 then
    return redis.error_reply('invalid waitingTTL: expected positive integer seconds')
end
local matched_ttl = tonumber(ARGV[3])
if matched_ttl == nil or matched_ttl <= 0 or matched_ttl % 1 ~= 0 then
    return redis.error_reply('invalid matchedTTL: expected positive integer seconds')
end
local key_prefix_partner_index = ARGV[4]

local partner_id = nil

while true do
    local potential_partner_id = redis.call('RPOP', partner_queue_key)
    if potential_partner_id == false then
        break
    end
    local potential_partner_index_key = key_prefix_partner_index .. potential_partner_id
    
    local potential_partner_state = redis.call('HGET', potential_partner_index_key, 'state')
    if potential_partner_state == 'waiting' then
        local potential_partner_queue_key = redis.call('HGET', potential_partner_index_key, 'queue_key')
        if partner_queue_key == potential_partner_queue_key then
            partner_id = potential_partner_id
            break
        end
    end
end

if not partner_id then
    redis.call('LPUSH', client_queue_key, client_id)
    redis.call('HSET', client_key, 'state', 'waiting', 'queue_key', client_queue_key)
    redis.call('EXPIRE', client_key, waiting_ttl)
    return {0}
else
    local partner_index_key = key_prefix_partner_index .. partner_id
    -- remove old partner data
    redis.call("DEL", partner_index_key)

    redis.call('HSET', client_key, 'state', 'matched', 'partner_client_id', partner_id, 'partner_queue_key', partner_queue_key)
    redis.call('HSET', partner_index_key, 'state', 'matched', 'partner_client_id', client_id, 'partner_queue_key', client_queue_key)
    redis.call('EXPIRE', client_key, matched_ttl)
    redis.call('EXPIRE', partner_index_key, matched_ttl)
    return {1, partner_id, partner_queue_key}
end
