#include "intcache.h"
#include "../evmone.release/cache.hpp"

typedef Cache<int64_t> IntCache;

const int SHARD_COUNT = 4;
IntCache shards[SHARD_COUNT];

void add(int64_t key, int64_t value, uint32_t height) {
	auto sid = key % SHARD_COUNT;
	shards[sid].add(std::to_string(key), value, height);
}

int64_t* borrow(int64_t key) {
	auto sid = key % SHARD_COUNT;
	const int64_t& res = shards[sid].borrow(std::to_string(key));
	return (int64_t*)&res;
}

uint64_t give_back(int64_t key, uint32_t height) {
	auto sid = key % SHARD_COUNT;
	shards[sid].give_back(std::to_string(key), height);
	return (shards[3].size()<<48)|(shards[2].size()<<32)|(shards[1].size()<<16)|shards[0].size();
}

