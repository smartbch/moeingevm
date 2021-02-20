#pragma once

#include <string.h>

inline uint64_t fasthash(const uint8_t arr[], size_t size) {
	uint64_t hash = 5381;
	for(size_t i = 0; i < size; i++) {
		hash = ((hash << 5) + hash) + uint64_t(arr[i]); /* hash * 33 + c */
	}
	return hash;
}

class hashfn_evmc_address {
public:
	size_t operator() (evmc_address const& key) const {
		return fasthash(&key.bytes[0], sizeof(evmc_address));
	}
};
class equalfn_evmc_address {
public:
	bool operator() (evmc_address const& a1, evmc_address const& a2) const {
		return memcmp(a1.bytes, a2.bytes, sizeof(evmc_address)) == 0;
	}
};

struct storage_key {
	uint64_t account_seq;
	evmc_bytes32 key;
};

inline storage_key skey(uint64_t account_seq, const evmc_bytes32& key) {
	storage_key result;
	result.account_seq = account_seq;
	result.key = key;
	return result;
}

class hashfn_storage_key {
public:
	size_t operator() (storage_key const& skey) const {
		return skey.account_seq ^ fasthash(&skey.key.bytes[0], sizeof(evmc_bytes32));
	}
};
class equalfn_storage_key {
public:
	bool operator() (storage_key const& k1, storage_key const& k2) const {
		return memcmp(k1.key.bytes, k2.key.bytes, sizeof(evmc_bytes32)) == 0 &&
			k1.account_seq == k2.account_seq;
	}
};

