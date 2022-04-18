#pragma once
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

	void add(int64_t key, int64_t value, uint32_t height);
	int64_t* borrow(int64_t key);
	uint64_t give_back(int64_t key, uint32_t height);

#ifdef __cplusplus
}
#endif

