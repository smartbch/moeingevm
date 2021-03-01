#include <stdio.h>
#include "bridge.h"

typedef int64_t (*zero_depth_call_func_t)(evmc_uint256be gas_price,
                     int64_t gas_limit,
                     const evmc_address* destination,
                     const evmc_address* sender,
                     const evmc_uint256be* value,
                     const uint8_t* input_data,
                     size_t input_size,
		     const struct block_info* block,
		     int handler,
		     bool need_gas_estimation,
		     bridge_get_creation_counter_fn get_creation_counter_fn,
		     bridge_get_account_info_fn get_account_info_fn,
		     bridge_get_bytecode_fn get_bytecode_fn,
		     bridge_get_value_fn get_value_fn,
		     bridge_get_block_hash_fn get_block_hash_fn,
		     bridge_collect_result_fn collect_result_fn,
		     bridge_call_precompiled_contract_fn call_precompiled_contract_fn);


zero_depth_call_func_t zero_depth_call_func;

int64_t zero_depth_call_wrap(evmc_bytes32 gas_price,
                             int64_t gas_limit,
                             const evmc_address* destination,
                             const evmc_address* sender,
                             const evmc_bytes32* value,
                             const uint8_t* input_data,
                             size_t input_size,
		             const struct block_info* block,
		             int collector_handler,
		             bool need_gas_estimation) {
	return zero_depth_call_func(gas_price,
                             gas_limit,
                             destination,
                             sender,
                             value,
                             input_data,
                             input_size,
		             block,
		             collector_handler,
		             need_gas_estimation,
	                     get_creation_counter,
	                     get_account_info,
	                     get_bytecode,
	                     get_value,
	                     get_block_hash,
	                     collect_result,
	                     call_precompiled_contract);
}

enum dl_init_status init_dl() {
	char* path = getenv("EVMWRAP");
	if (strlen(path) == 0) {
		return ENV_NOT_DEFINED;
	}

	void* lib_handle = dlopen(path, RTLD_LAZY );
	if (!lib_handle){
		return FAIL_TO_OPEN;
	}
 
	zero_depth_call_func = (zero_depth_call_func_t)dlsym(lib_handle, "zero_depth_call");
	char* errorInfo = dlerror();
	if (errorInfo != NULL) {
		return SYMBOL_NOT_FOUND;
	}
	
        return OK;
}
 
