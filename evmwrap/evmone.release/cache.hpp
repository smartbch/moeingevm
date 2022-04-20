#pragma once
#include <string>
#include <mutex>
#include <unordered_map>

typedef unsigned __int128 uint128_t;

template <typename Payload>
class Cache {
	struct PayloadAndHistory {
		Payload payload;
		uint128_t history;
	};

	std::mutex mtx;
	typedef std::unordered_map<std::string, PayloadAndHistory> map_t;
	map_t m;
	Payload empty_payload;
	int max_size;

	void evict_from(typename map_t::iterator iter) {
		std::string key_to_evict = "";
		uint64_t min_score = ~uint64_t(0); // maximum value of uint64_t 
		for(int i=0; i<STRIDE_SIZE; i++, iter++) { // within a stride, find a least-recently-used key to evict
			if(iter == m.end()) { //loop back to the beginning
				iter = m.begin();
			}
			uint128_t h = iter->second.history;
			if(uint32_t(h) > MAX_INT32) { //it's in use
				continue; //ignore it
			}
			// sum the heights of last four accesses as the score
			uint64_t score = uint64_t(uint32_t(h>>96) + uint32_t(h>>64)) + 
					 uint64_t(uint32_t(h>>32) + uint32_t(h));
			if(min_score > score) { // record the entry with smallest score for later evicting
				min_score = score;
				key_to_evict = iter->first;
			}
		}
		if(key_to_evict.size() != 0) { // evict the recorded entry
			m[key_to_evict].payload = empty_payload; //clear payload for easier debug
			m.erase(key_to_evict);
		}
	}
public:
	static const int STRIDE_SIZE = 10;
	static const int DEFAULT_MAX_SIZE = 100;
	static const int SHARD_COUNT = 16;
	static const uint32_t MAX_INT32 = 0x7FFFFFFF;
	Cache(): mtx(), m(), empty_payload(), max_size(DEFAULT_MAX_SIZE) {}

	// given a key and find the cached payload
	const Payload& borrow(const std::string& key) {
		mtx.lock();
		auto iter = m.find(key);
		if(iter == m.end()) { // if nothing is found, return an empty payload
			mtx.unlock();
			return empty_payload;
		}
		uint32_t latest_slot = uint32_t(iter->second.history);
		if(latest_slot <= MAX_INT32) { //not in use
			// insert 0x80000000 to the latest slot, which marks this entry as "in use"
			iter->second.history = (iter->second.history<<32) | uint128_t(MAX_INT32+1);
		} else { // in use
			// increase reference count
			iter->second.history += 1;
		}
		const Payload& res = iter->second.payload;
		if(m.size() > max_size) { // if cache is full, evict some entries out
			evict_from(++iter);
		}
		mtx.unlock();
		return res;
	}

	// return an entry to the cache
	void give_back(const std::string& key, uint32_t height) {
		mtx.lock();
		auto iter = m.find(key);
		if(iter != m.end()) {
			// decrease reference count
			iter->second.history -= 1;
			uint32_t latest_slot = uint32_t(iter->second.history);
			if(latest_slot == MAX_INT32) { // the reference count is zero now
				// set the latest slot as a real height
				iter->second.history = (iter->second.history^uint128_t(latest_slot)) & uint128_t(height);
			}
		}
		mtx.unlock();
	}

	// add a new entry to the cache. the content of 'payload' will be moved into the cached entry
	void add(const std::string& key, Payload& payload, uint32_t height) {
		mtx.lock();
		auto h = uint128_t(height);
		uint128_t history = (h<<96)|(h<<64)|(h<<32)|h; //fill all the four slots with the current height
		m.emplace(key, PayloadAndHistory{payload: std::move(payload), history: history}); // add a new entry
		mtx.unlock();
	}

	size_t size() {
		return m.size();
	}
};
