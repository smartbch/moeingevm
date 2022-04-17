#include "intcache.h"
#include "../evmone.release/cache.hpp"

typedef Cache<int64_t> IntCache;

IntCache shards[IntCache::SHARD_COUNT];

void add(int64_t key, int64_t value, uint32_t height) {
	auto sid = key % IntCache::SHARD_COUNT;
	shards[sid].add(std::to_string(key), value, height);
}

int64_t* borrow(int64_t key) {
	auto sid = key % IntCache::SHARD_COUNT;
	const int64_t& res = shards[sid].borrow(std::to_string(key));
	return (int64_t*)&res;
}

void give_back(int64_t key, uint32_t height) {
	auto sid = key % IntCache::SHARD_COUNT;
	shards[sid].give_back(std::to_string(key), height);
}

