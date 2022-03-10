package ebp

//#cgo LDFLAGS: -l:libevmwrap.a -L../evmwrap/host_bridge -lstdc++
//#include "../evmwrap/host_bridge/bridge.h"
//#include <dlfcn.h>
//#include <stdlib.h>
//
//enum dl_init_status {
//	OK = 0,
//	ENV_NOT_DEFINED = 1,
//	FAIL_TO_OPEN = 2,
//	SYMBOL_NOT_FOUND = 3
//};
//
//enum dl_init_status init_dl();
//
//// Below we declare some extern functions, whose definition is the exported functions in ebp.go
//extern uint64_t get_creation_counter(int handler, uint8_t);
//extern void get_account_info(int handler,
//                             struct evmc_address* addr,
//                             struct evmc_bytes32* balance,
//                             uint64_t* nonce,
//                             uint64_t* sequence);
//extern void get_bytecode(int handler,
//                         struct evmc_address* addr,
//                         struct evmc_bytes32* codehash_out,
//                         struct big_buffer* buf,
//                         size_t* size);
//extern void get_value(int handler,
//                      uint64_t acc_sequence,
//                      char* key_ptr,
//                      struct big_buffer* buf,
//                      size_t* size);
//extern evmc_bytes32 get_block_hash(int handler, uint64_t num);
//extern void collect_result(int handler, struct all_changed* result, struct evmc_result* ret_value);
//extern void call_precompiled_contract (struct evmc_address* contract_addr,
//                                       void* input_ptr,
//                                       int input_size,
//                                       uint64_t* gas_left,
//                                       int* ret_value,
//                                       int* out_of_gas,
//                                       struct small_buffer* output_ptr,
//                                       int* output_size);
//
//int64_t zero_depth_call_wrap(evmc_bytes32 gas_price,
//                             int64_t gas_limit,
//                             const evmc_address* destination,
//                             const evmc_address* sender,
//                             const evmc_bytes32* value,
//                             const uint8_t* input_data,
//                             size_t input_size,
//                             const struct block_info* block,
//                             int collector_handler,
//                             bool need_gas_estimation,
//                             enum evmc_revision revision) {
//       return zero_depth_call(gas_price,
//                             gas_limit,
//                             destination,
//                             sender,
//                             value,
//                             input_data,
//                             input_size,
//                             block,
//                             collector_handler,
//                             need_gas_estimation,
//                             revision,
//                             get_creation_counter,
//                             get_account_info,
//                             get_bytecode,
//                             get_value,
//                             get_block_hash,
//                             collect_result,
//                             call_precompiled_contract);
//}
import "C"
