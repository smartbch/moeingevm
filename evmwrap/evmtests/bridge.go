package main

/*
#cgo linux LDFLAGS: -l:libevmwrap.a -L../host_bridge -lstdc++ -ldl
#cgo darwin LDFLAGS: -levmwrap -L../host_bridge -lstdc++ -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>
#include "../host_bridge/bridge.h"

enum dl_init_status {
	OK = 0,
	FAIL_TO_OPEN = 1,
	SYMBOL_NOT_FOUND = 2,
	FAIL_TO_CLOSE = 3
};

// Below we declare some extern functions, whose definition is the exported functions in ebp.go
extern uint64_t get_creation_counter(int handler, uint8_t);
extern void get_account_info(int handler,
                             struct evmc_address* addr,
                             struct evmc_bytes32* balance,
                             uint64_t* nonce,
                             uint64_t* sequence);
extern void get_bytecode(int handler,
                         struct evmc_address* addr,
                         struct evmc_bytes32* codehash_out,
                         struct big_buffer* buf,
                         size_t* size);
extern void get_value(int handler,
                      uint64_t acc_sequence,
                      char* key_ptr,
                      struct big_buffer* buf,
                      size_t* size);
extern evmc_bytes32 get_block_hash(int handler, uint64_t num);
extern void collect_result(int handler, struct all_changed* result, struct evmc_result* ret_value);
extern void call_precompiled_contract (struct evmc_address* contract_addr,
                                       void* input_ptr,
                                       int input_size,
                                       uint64_t* gas_left,
                                       int* ret_value,
                                       int* out_of_gas,
                                       struct small_buffer* output_ptr,
                                       int* output_size);

int64_t zero_depth_call_wrap(evmc_bytes32 gas_price,
                             int64_t gas_limit,
                             const evmc_address* destination,
                             const evmc_address* sender,
                             const evmc_bytes32* value,
                             const uint8_t* input_data,
                             size_t input_size,
                             const struct block_info* block,
                             int collector_handler,
                             bool need_gas_estimation,
                             enum evmc_revision revision,
		             bridge_query_executor_fn query_executor_fn) {
       return zero_depth_call(gas_price,
                             gas_limit,
                             destination,
                             sender,
                             value,
                             input_data,
                             input_size,
                             block,
                             collector_handler,
                             need_gas_estimation,
                             revision,
                             query_executor_fn,
                             get_creation_counter,
                             get_account_info,
                             get_bytecode,
                             get_value,
                             get_block_hash,
                             collect_result,
                             call_precompiled_contract);
}

bridge_query_executor_fn load_func_from_dl(_GoString_ path, int* status) {
	static void* lib_handle = NULL;

        if (lib_handle && !dlclose(lib_handle)) {
                *status = FAIL_TO_CLOSE;
		return NULL;
        }
        lib_handle = dlopen(_GoStringPtr(path), RTLD_LAZY );
        if (!lib_handle){
                *status = FAIL_TO_OPEN;
		return NULL;
        }

        bridge_query_executor_fn f = (bridge_query_executor_fn)dlsym(lib_handle, "query_executor");
        char* errorInfo = dlerror();
        if (errorInfo != NULL) {
                *status = SYMBOL_NOT_FOUND;
		return NULL;
        }

        *status = OK;
	return f;
}
*/
import "C"
import (
	"os"
	"path"
	"strings"
)

var (
	QueryExecutorFn C.bridge_query_executor_fn
	lastFile        string
)

func ReloadQueryExecutorFn(aotDir string) {
	files, err := os.ReadDir(aotDir)
	if err != nil {
		panic(err)
	}

	libFiles := make([]string, 0, len(files))
	for _, entry := range files {
		if strings.HasPrefix(entry.Name(), "lib") {
			libFiles = append(libFiles, entry.Name())
		}
	}

	if len(libFiles) == 0 {
		return
	}

	libFile := path.Join(aotDir, libFiles[len(libFiles)-1])
	if libFile == lastFile {
		return
	}
	lastFile = libFile

	var status C.int
	QueryExecutorFn = C.load_func_from_dl(libFile+"\x00", &status)
	if status != 0 {
		panic("Failed to load dynamic library")
	}
}
