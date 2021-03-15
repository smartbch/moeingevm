#include <evmone/evmone.h>
#include <array>
#include <iostream>
#include "host_context.h"
extern "C" {
#include "../sha256/sha256.h"
#include "../ripemd160/ripemd160.h"
}

bool is_precompiled(const evmc_address& addr) {
	for(int i=0; i<19; i++) {
		if(addr.bytes[i] != 0) return false;
	}
	return 1<=addr.bytes[19] && addr.bytes[19]<=9;
}

// following functions wrap C++ member functions into C-style functions, thus
// we can build the virtual function table evmc_host_interface
struct evmc_tx_context evmc_get_tx_context(struct evmc_host_context* context) {
	return context->get_tx_context();
}

evmc_bytes32 evmc_get_block_hash(struct evmc_host_context* context, int64_t number) {
	return context->get_block_hash(number);
}

bool evmc_account_exists(struct evmc_host_context* context, const evmc_address* address) {
	return context->account_exists(*address);
}

evmc_bytes32 evmc_get_storage(struct evmc_host_context* context,
                              const evmc_address* address,
                              const evmc_bytes32* key) {
	return context->get_storage(*address, *key);
}

enum evmc_storage_status evmc_set_storage(struct evmc_host_context* context,
                                          const evmc_address* address,
                                          const evmc_bytes32* key,
                                          const evmc_bytes32* value) {
	return context->set_storage(*address, *key, *value);
}

evmc_uint256be evmc_get_balance(struct evmc_host_context* context,
                                const evmc_address* address) {
	return context->get_balance(*address);
}

size_t evmc_get_code_size(struct evmc_host_context* context,
                          const evmc_address* address) {
	return context->get_code_size(*address);
}

evmc_bytes32 evmc_get_code_hash(struct evmc_host_context* context,
                                const evmc_address* address) {
	return context->get_code_hash(*address);
}

size_t evmc_copy_code(struct evmc_host_context* context,
                      const evmc_address* address,
                      size_t code_offset,
                      uint8_t* buffer_data,
                      size_t buffer_size) {
	return context->copy_code(*address, code_offset, buffer_data, buffer_size);
}

void evmc_selfdestruct(struct evmc_host_context* context,
                       const evmc_address* address,
                       const evmc_address* beneficiary) {
	return context->selfdestruct(*address, *beneficiary);
}

void evmc_emit_log(struct evmc_host_context* context,
                   const evmc_address* address,
                   const uint8_t* data,
                   size_t data_size,
                   const evmc_bytes32 topics[],
                   size_t topics_count) {
	return context->emit_log(*address, data, data_size, topics, topics_count);
}

struct evmc_result evmc_call(struct evmc_host_context* context,
                             const struct evmc_message* msg) {
	return context->call(*msg);
}

evmc_host_interface HOST_IFC {
	.account_exists = evmc_account_exists,
	.get_storage = evmc_get_storage,
	.set_storage = evmc_set_storage,
	.get_balance = evmc_get_balance,
	.get_code_size = evmc_get_code_size,
	.get_code_hash = evmc_get_code_hash,
	.copy_code = evmc_copy_code,
	.selfdestruct = evmc_selfdestruct,
	.call = evmc_call
	.get_tx_context = evmc_get_tx_context,
	.get_block_hash = evmc_get_block_hash,
	.emit_log = evmc_emit_log,
};


evmc_bytes32 ZERO_BYTES32 {
	.bytes = {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
};

// Following is a C++ implementation of RLP, according to the python code in https://eth.wiki/fundamentals/rlp

//def to_binary(x):
//    if x == 0:
//        return ''
//    else:
//        return to_binary(int(x / 256)) + chr(x % 256)
bytes rlp_encode_uint(size_t x) {
	size_t num_bytes = 0;
	for(size_t y = x; y != 0; y = y>>8) {
		num_bytes++;
	}
	bytes result;
	result.reserve(num_bytes);
	for(int i = num_bytes - 1; i >= 0; i--) {
		result.append(1, char(x>>(i*8)));
	}
	return result;
}

//def encode_length(L,offset):
//    if L < 56:
//         return chr(L + offset)
//    elif L < 256**8:
//         BL = to_binary(L)
//         return chr(len(BL) + offset + 55) + BL
//    else:
//         raise Exception("input too long")
bytes rlp_encode_length(size_t L, size_t offset) {
	if(L < 56) return bytes(1, L+offset);
	bytes BL = rlp_encode_uint(L);
	return bytes(1, BL.size()+offset+55) + BL;
}

//def rlp_encode(input):
//    if isinstance(input,str):
//        if len(input) == 1 and ord(input) < 0x80: return input
//        else: return encode_length(len(input), 0x80) + input
//    elif isinstance(input,list):
//        output = ''
//        for item in input: output += rlp_encode(item)
//        return encode_length(len(output), 0xc0) + output
//
bytes rlp_encode(const bytes& input) {
	if(input.size() == 1 && input[0] < 0x80) return input;
	return rlp_encode_length(input.size(), 0x80) + input;
}
bytes rlp_encode(const std::vector<bytes>& input_vec) {
	bytes output;
	for(auto& elem : input_vec) {
		output += rlp_encode(elem);
	}
	return rlp_encode_length(output.size(), 0xc0) + output;
}

bytes rlp_encode_bytes(const uint8_t* data, size_t size) {
	size_t start = 0;
	for(; start < size; start++) { // the leading zeros are ignored
		if(data[start] != 0) break;
	}
	bytes result;
	result.reserve(size - start + 1);
	for(; start < size; start++) {
		result.append(1, char(data[start]));
	}
	return result;
}

// calculate the created contract's address of CREATE instruction
evmc_address create_contract_addr(const evmc_address& creater, uint64_t nonce) {
	//std::cerr<<"create_contract_addr "<<to_hex(creater)<<" "<<nonce<<std::endl;
	std::vector<bytes> str_vec(2);
	str_vec[0] = rlp_encode_bytes(&creater.bytes[0], sizeof(evmc_address));
	str_vec[1] = rlp_encode_uint(nonce);
	auto s = rlp_encode(str_vec);
	ethash::hash256 hash = ethash::keccak256(s.c_str(), s.size());
	evmc_address result;
	memcpy(&result.bytes[0], &hash.bytes[12], 20);
	return result;
}

// calculate the created contract's address of CREATE2 instruction
evmc_address create2_contract_addr(const evmc_address& creater, const evmc_bytes32& salt, const evmc_bytes32& codehash) {
	uint8_t arr[1+sizeof(evmc_address)+sizeof(evmc_bytes32)*2];
	arr[0] = 0xff;
	memcpy(arr+1, &creater.bytes[0], sizeof(evmc_address));
	memcpy(arr+1+sizeof(evmc_address), &salt.bytes[0], sizeof(evmc_bytes32));
	memcpy(arr+1+sizeof(evmc_address)+sizeof(evmc_bytes32), &codehash.bytes[0], sizeof(evmc_bytes32));
	evmc_address result;
	ethash::hash256 hash = ethash::keccak256(arr, sizeof(arr));
	memcpy(&result.bytes[0], &hash.bytes[12], 20);
	return result;
}

bool bytes32_equal(const evmc_bytes32& a, const evmc_bytes32& b) {
	return memcmp(&a.bytes[0], &b.bytes[0], 32) == 0;
}

bool address_equal(const evmc_address& a, const evmc_address& b) {
	return memcmp(&a.bytes[0], &b.bytes[0], 20) == 0;
}

// for EXTCODESIZE
size_t evmc_host_context::get_code_size(const evmc_address& addr) {
	const account_info& info = txctrl->get_account(addr);
	if(info.is_null() || info.is_empty()) {
		return 0;
	}
	if(info.sequence == ~0) { // before set_bytecode or EOA
		return 0;
	}
	return txctrl->get_bytecode_entry(addr).bytecode.size();
}

// for EXTCODEHASH
const evmc_bytes32& evmc_host_context::get_code_hash(const evmc_address& addr) {
	const account_info& info = txctrl->get_account(addr);
	//std::cerr<<"ADDR gete_code_hash "<<to_hex(addr)<<" selfdestruct "<<info.selfdestructed<<std::endl;
	if(info.is_null()) {
		return ZERO_BYTES32;
	}
	auto& entry = txctrl->get_bytecode_entry(addr);
	if(info.nonce == 0 && info.balance == uint256(0) && entry.bytecode.size() == 0) {
		return ZERO_BYTES32;
	}
	return txctrl->get_bytecode_entry(addr).codehash;
}

// for EXTCODECOPY
size_t evmc_host_context::copy_code(const evmc_address& addr, size_t code_offset, uint8_t* buffer_data, size_t buffer_size) {
	const bytes& src = txctrl->get_bytecode_entry(addr).bytecode;
	if(code_offset >= src.size()) {
		return 0;
	}
	size_t num_bytes = src.size() - code_offset;
	if(num_bytes > buffer_size) {
		num_bytes = buffer_size;
	}
	memcpy(buffer_data, src.data() + code_offset, num_bytes);
	return num_bytes;
}

// for SELFDESTRUCT
void evmc_host_context::selfdestruct(const evmc_address& addr, const evmc_address& beneficiary) {
	if(!txctrl->is_selfdestructed(addr)) {
		txctrl->add_refund(SELFDESTRUCT_REFUND_GAS);
	}
	uint256 balance = txctrl->get_balance(addr); //make a copy

	const account_info& acc = txctrl->get_account(beneficiary);
	bool is_prec = is_precompiled(beneficiary) && SELFDESTRUCT_BENEFICIARY_CANNOT_BE_PRECOMPILED;
	bool zero_value = balance == uint256(0);
	equalfn_evmc_address equalfn;
	bool self_as_beneficiary = equalfn(beneficiary, this->msg.destination);
	if(acc.is_null() && !is_prec && !zero_value) {
		//std::cerr<<"HERE create "<<to_hex(beneficiary)<<std::endl;
		txctrl->new_account(beneficiary);
	}
	bool is_empty = (acc.nonce == 0 && acc.balance == uint256(0) &&
			txctrl->get_bytecode_entry(beneficiary).bytecode.size() == 0);
	//std::cerr<<"try eip158 in selfdestruct"<<is_empty<<" "<<zero_value<<std::endl;
	if(is_empty && zero_value) {
		txctrl->selfdestruct(beneficiary); //eip158
	}

	if(self_as_beneficiary) {
		txctrl->burn(addr, balance);
	} else if(!is_prec && !self_as_beneficiary) {
		txctrl->transfer(addr, beneficiary, balance);
	}

	//std::cerr<<"HERE from "<<to_hex(addr)<<" "<<intx::to_string(txctrl->get_balance(addr))<<std::endl;
	//std::cerr<<"HERE to "<<to_hex(beneficiary)<<" "<<intx::to_string(txctrl->get_balance(beneficiary))<<std::endl;
	txctrl->selfdestruct(addr);
}

// load bytecode into this->code before running it
void evmc_host_context::load_code(const evmc_address& addr) {
	const account_info& acc = txctrl->get_account(msg.destination);
	if(acc.is_null()) {
		this->code = &this->empty_code;
	}
	this->code = &txctrl->get_bytecode_entry(addr).bytecode;
}

evmc_result evmc_host_context::call(const evmc_message& call_msg) {
	//std::cerr<<"NOW call with gas "<<call_msg->gas<<std::endl;
	txctrl->gas_trace_append(call_msg.gas|MSB64);
	evmc_host_context ctx(txctrl, call_msg, this->smallbuf);
	evmc_result result;
	bool normal_run = false;
	switch (call_msg.kind) {
	case EVMC_CALL:
		if((call_msg.flags & EVMC_STATIC) != 0) {
			normal_run = true;
		} else {
			result = ctx.call();
		}
		break;
	case EVMC_CALLCODE:
	case EVMC_DELEGATECALL:
		ctx.msg.destination = msg.destination;
		//std::cerr<<"CALLCODE/DELEGATECALL src "<<to_hex(call_msg.sender)<<" dst "<<to_hex(call_msg.destination)<<" self "<<to_hex(ctx.msg.destination)<<std::endl;
		normal_run = true;
		break;
	case EVMC_CREATE:
		result = ctx.create();
		break;
	case EVMC_CREATE2:
		result = ctx.create2();
		break;
	default:
		assert(false);
	}
	if(normal_run) {
		if(is_precompiled(call_msg.destination)) {
			result = ctx.run_precompiled_contract(call_msg.destination);
		} else {
			ctx.load_code(call_msg.destination);
			if(call_msg.kind == EVMC_CALL) {
				ctx.check_eip158();
			}
			result = ctx.run_vm(txctrl->snapshot());
		}
	}
	//std::cerr<<"NOW return with gas_left "<<result.gas_left<<std::endl;
	txctrl->gas_trace_append(result.gas_left);
	return result;
}

// EIP158 request us to delete empty accounts when they are "touched"
void evmc_host_context::check_eip158() {
	const account_info& acc = txctrl->get_account(msg.destination);
	bool zero_value = is_zero_bytes32(&msg.value);
	bool is_empty = (acc.nonce == 0 && acc.balance == uint256(0) && this->code->size() == 0);
	//std::cerr<<"try2 eip158 "<<is_empty<<" "<<zero_value<<std::endl;
	if(is_empty && zero_value) {
		txctrl->selfdestruct(msg.destination); //eip158
	}
}

evmc_result evmc_host_context::call() {
	//std::cerr<<"$msg.gas "<<msg.gas<<std::endl;
	const account_info& acc = txctrl->get_account(msg.destination);
	bool zero_value = is_zero_bytes32(&msg.value);
	bool call_precompiled = is_precompiled(msg.destination);
	size_t snapshot = txctrl->snapshot();
	load_code(msg.destination);
	bool is_empty = (acc.nonce == 0 && acc.balance == uint256(0) && this->code->size() == 0);
	if(acc.is_null() /*&& !call_precompiled*/) {
		if(zero_value && !call_precompiled) {
			return evmc_result {.status_code=EVMC_SUCCESS, .gas_left=msg.gas};
		}
		txctrl->new_account(msg.destination);
		//std::cerr<<"$msg.gas after create "<<msg.gas<<std::endl;
	}
	//std::cerr<<"try eip158 "<<is_empty<<" "<<zero_value<<" "<<to_hex(msg.destination)<<std::endl;
	if(is_empty && zero_value) { //eip158
		txctrl->selfdestruct(msg.destination);
	}
	//std::cerr<<"Sender "<<to_hex(msg.sender)<<" Dst "<<to_hex(msg.destination)<<std::endl;
	if(!zero_value /*&& !call_precompiled*/) {
		txctrl->transfer(msg.sender, msg.destination, u256be_to_u256(msg.value));
	}
	evmc_result result;
	if(call_precompiled) {
		result = run_precompiled_contract(msg.destination);
		if(result.status_code != EVMC_SUCCESS) {
			txctrl->revert_to_snapshot(snapshot);
		}
	} else {
		result = run_vm(snapshot);
	}
	
	//std::cerr<<" result.gas_left "<<result.gas_left<<std::endl;
	return result;
}

evmc_result evmc_host_context::run_precompiled_contract(const evmc_address& addr) {
	if(addr.bytes[0] == 2) {
		auto gas = (msg.input_size+31)/32*SHA256_PER_WORD_GAS + SHA256_BASE_GAS;
		if(gas > msg.gas) {
			return evmc_result{.status_code=EVMC_OUT_OF_GAS};
		}
		SHA256_CTX ctx;
		sha256_init(&ctx);
		sha256_update(&ctx, msg.input_data, msg.input_size);
		sha256_final(&ctx, (uint8_t*)&this->smallbuf->data[0]);
		return evmc_result{
			.status_code=EVMC_SUCCESS,
			.gas_left=int64_t(msg.gas-gas),
			.output_data=(uint8_t*)&this->smallbuf->data[0],
			.output_size=SHA256_BLOCK_SIZE};
	} else if(addr.bytes[0] == 3) {
		auto gas = (msg.input_size+31)/32*RIPEMD160_PER_WORD_GAS + RIPEMD160_BASE_GAS;
		if(gas > msg.gas) {
			return evmc_result{.status_code=EVMC_OUT_OF_GAS};
		}
		ripemd160(msg.input_data, msg.input_size, (uint8_t*)&this->smallbuf->data[0]);
		return evmc_result{
			.status_code=EVMC_SUCCESS,
			.gas_left=int64_t(msg.gas-gas),
			.output_data=(uint8_t*)&this->smallbuf->data[0],
			.output_size=RIPEMD160_DIGEST_LENGTH};
	} else if(addr.bytes[0] == 4) {
		auto gas = (msg.input_size+31)/32*IDENTITY_PER_WORD_GAS + IDENTITY_BASE_GAS;
		if(gas > msg.gas) {
			return evmc_result{.status_code=EVMC_OUT_OF_GAS};
		}
		return evmc_result{
			.status_code=EVMC_SUCCESS,
			.gas_left=int64_t(msg.gas-gas),
			.output_data=msg.input_data,
			.output_size=msg.input_size};
	}
	// the others use golang implementations
	int ret_value, out_of_gas, osize;
	uint64_t gas_left = msg.gas;

	//std::cerr<<"@@inputdata "<<msg.input_size<<" ptr "<<size_t(msg.input_data)<<" ";
	//for(int i = 0; i < msg.input_size; i++) {
	//    std::cerr<<std::hex<<" "<<int(msg.input_data[i]);
	//}
	//std::cerr<<std::dec<<std::endl;

	this->txctrl->call_precompiled_contract((struct evmc_address*)&addr/*drop const*/, (void*)msg.input_data, msg.input_size,
	                                        &gas_left, &ret_value, &out_of_gas, this->smallbuf, &osize);
	if(out_of_gas != 0) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	if(ret_value != 1) {
		return evmc_result{.status_code=EVMC_FAILURE};
	}
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(gas_left),
		.output_data=(uint8_t*)&this->smallbuf->data[0],
		.output_size=uint64_t(osize)};
}

evmc_result evmc_host_context::run_vm(size_t snapshot) {
	//std::cerr<<"code size "<<this->code->size()<<std::endl;
	if(this->code->size() == 0) {
		return evmc_result{.status_code=EVMC_SUCCESS, .gas_left=msg.gas}; // do nothing
	}
	evmc_result result = txctrl->execute(nullptr, &HOST_IFC, this, EVMC_MAX_REVISION, &msg,
			this->code->data(), this->code->size());
	//std::cerr<<"result.status_code "<<result.status_code<<std::endl;
	if(result.status_code != EVMC_SUCCESS) {
		txctrl->revert_to_snapshot(snapshot);
	}
	return result;
}

evmc_bytes32 keccak256(const uint8_t* data, size_t size) {
	evmc_bytes32 result;
	auto hash = ethash::keccak256(data, size);
	memcpy(&result.bytes[0], &hash.bytes[0], sizeof(evmc_bytes32));
	return result;
}

evmc_result evmc_host_context::create() {
	auto& acc = txctrl->get_account(msg.sender);
	uint64_t nonce = acc.nonce;
	if(msg.depth == 0) nonce--;
	evmc_address addr = create_contract_addr(msg.sender, nonce);
	return create_with_contract_addr(addr);
}

evmc_result evmc_host_context::create2() {
	evmc_bytes32 codehash = keccak256(msg.input_data, msg.input_size);
	evmc_address addr = create2_contract_addr(msg.sender, msg.create2_salt, codehash);
	return create_with_contract_addr(addr);
}

bool evmc_host_context::create_pre_check(const evmc_address& new_addr) {
	auto& acc = txctrl->get_account(new_addr);
	auto& codehash = this->get_code_hash(new_addr);
	if(!acc.is_null() && (acc.nonce != 0 ||
				(!bytes32_equal(codehash, ZERO_BYTES32) &&
				 !bytes32_equal(codehash, HASH_FOR_ZEROCODE) ) ) ) {
		return false;
	}
	return true;
}

evmc_result evmc_host_context::create_with_contract_addr(const evmc_address& addr) {
	if(msg.depth != 0) {
		txctrl->incr_nonce(msg.sender);
	}
	if(!create_pre_check(addr)) {
		return evmc_result{.status_code = EVMC_FAILURE, .gas_left = 0};
	}
	msg.destination = addr;
	bytes input_as_code(msg.input_data, msg.input_size);
	this->code = &input_as_code;
	msg.input_size = 0;

	size_t snapshot = txctrl->snapshot(); //if failed, revert account creation
	if(txctrl->get_account(addr).is_null() || txctrl->get_account(addr).is_empty()) {
		txctrl->new_account(addr);
	}
	txctrl->incr_nonce(addr);
	txctrl->set_bytecode(addr, bytes(), HASH_FOR_ZEROCODE);
	txctrl->transfer(msg.sender, addr, u256be_to_u256(msg.value));

	evmc_result result = this->run_vm(snapshot);
	if(result.status_code == EVMC_REVERT) {
		return result;
	}

	bool max_code_size_exceed = result.output_size > MAX_CODE_SIZE;
	if(result.status_code == EVMC_SUCCESS && !max_code_size_exceed) {
		auto create_data_gas = result.output_size * CREATE_DATA_GAS;
		if(result.gas_left >= create_data_gas) {
			result.gas_left -= create_data_gas;
			evmc_bytes32 codehash = keccak256(result.output_data, result.output_size);
			txctrl->update_bytecode(addr, bytes(result.output_data, result.output_size), codehash);
		} else {
			result.status_code = EVMC_OUT_OF_GAS;
			result.gas_left = 0;
			//std::cerr<<"OOG After Init Code"<<std::endl;
		}
		result.output_size = 0; //the output is used as code, not returndata
	}
	if(max_code_size_exceed || (result.status_code != EVMC_SUCCESS /*&& 
		result.status_code != EVMC_OUT_OF_GAS*/)) {
		txctrl->revert_to_snapshot(snapshot);
		if(result.status_code != EVMC_REVERT) {
			result.gas_left = 0;
		}
	}
	if(result.status_code == EVMC_SUCCESS && max_code_size_exceed) {
		result.status_code = EVMC_FAILURE;
	}
	result.create_address = addr;
	return result;
}

bool is_zero_address(const evmc_address addr) {
	for(int i = 0; i < sizeof(evmc_address); i++) {
		if(addr.bytes[i] != 0) return false;
	}
	return true;
}

// intrinsic gas is the gas consumed before starting EVM
uint64_t intrinsic_gas(const uint8_t* input_data, size_t input_size, bool is_contract_creation) {
	uint64_t gas = TX_GAS;
	if(is_contract_creation) {
		gas = TX_GAS_CONTRACT_CREATION;
	}
	if(input_size == 0) {
		return gas;
	}
	size_t nz = 0;
	for(int i = 0; i < input_size; i++) {
		if(input_data[i] != 0) {
			nz++;
		}
	}
	if((MAX_UINT64-gas)/TX_DATA_NON_ZERO_GAS < nz) {
		return MAX_UINT64;
	}
	gas += nz * TX_DATA_NON_ZERO_GAS;

	size_t z = input_size - nz;
	if((MAX_UINT64-gas)/TX_DATA_ZERO_GAS < z) {
		return MAX_UINT64;
	}
	gas += z * TX_DATA_ZERO_GAS;
	return gas;
}

int64_t zero_depth_call(evmc_uint256be gas_price,
                     int64_t gas_limit,
                     const evmc_address* destination,
                     const evmc_address* sender,
                     const evmc_uint256be* value,
                     const uint8_t* input_data,
                     size_t input_size,
		     const block_info* block,
		     int handler,
		     bool need_gas_estimation,
		     bridge_get_creation_counter_fn get_creation_counter_fn,
		     bridge_get_account_info_fn get_account_info_fn,
		     bridge_get_bytecode_fn get_bytecode_fn,
		     bridge_get_value_fn get_value_fn,
		     bridge_get_block_hash_fn get_block_hash_fn,
		     bridge_collect_result_fn collect_result_fn,
		     bridge_call_precompiled_contract_fn call_precompiled_contract_fn) {

	std::array<big_buffer, 1> bigbuf;
	auto r = world_state_reader {
		.get_creation_counter_fn = get_creation_counter_fn,
		.get_account_info_fn = get_account_info_fn,
		.get_bytecode_fn = get_bytecode_fn,
		.get_value_fn = get_value_fn,
		.get_block_hash_fn = get_block_hash_fn,
		.bigbuf = &bigbuf[0],
		.handler = handler
	};
	bool is_contract_creation = is_zero_address(*destination);
	uint64_t intrinsic = intrinsic_gas(input_data, input_size, is_contract_creation);
	if(is_contract_creation && intrinsic > gas_limit) {
		// thus we can create zero account (TransactionSendingToZero)
		uint64_t no_create_gas = intrinsic_gas(input_data, input_size, false);
		if (no_create_gas <= gas_limit) {
			intrinsic = no_create_gas;
			is_contract_creation = false;
		}
	}
	if(intrinsic > gas_limit) {
		evmc_result result = {.status_code=EVMC_OUT_OF_GAS, .gas_left=0};
		collect_result_fn(handler, nullptr, &result);
		return 0;
	}
	//std::cerr<<"intrinsic_gas: "<<gas_limit<<"-"<<intrinsic<<std::endl;;
	gas_limit -= intrinsic;
	//std::cerr<<"="<<gas_limit<<std::endl;

	auto tx_context = evmc_tx_context {
		.tx_gas_price = gas_price,
		.tx_origin = *sender,
		.block_coinbase = block->coinbase,
		.block_number = block->number,
		.block_timestamp = block->timestamp,
		.block_gas_limit = block->gas_limit,
		.block_difficulty = block->difficulty,
		.chain_id = block->chain_id
	};
	auto msg = evmc_message {
		.kind = is_contract_creation? EVMC_CREATE : EVMC_CALL,
		.flags = 0,
		.depth = 0,
		.gas = gas_limit,
		.destination = *destination,
		.sender = *sender,
		.input_data = input_data,
		.input_size = input_size,
		.value = *value
	};
	evmc_vm* vm = evmc_create_evmone();

	tx_control txctrl(&r, tx_context, vm->execute, call_precompiled_contract_fn, need_gas_estimation);
	small_buffer smallbuf;
	evmc_host_context ctx(&txctrl, msg, &smallbuf);
	uint256 balance = ctx.get_balance_as_uint256(*sender);
	if(balance < u256be_to_u256(*value)) {
		//std::cerr<<"BAL not enough "<<intx::to_string(balance)<<" "<<intx::to_string(u256be_to_u256(*value))<<std::endl;
		evmc_result result = {.status_code=EVMC_FAILURE, .gas_left=msg.gas};
		txctrl.collect_result(collect_result_fn, handler, &result);
		vm->destroy(vm);
		return 0;
	}
	evmc_result result = ctx.call(msg);
	//std::cerr<<"result.status_code "<<result.status_code<<std::endl;
	txctrl.collect_result(collect_result_fn, handler, &result);
	int64_t gas_estimated = 0;
	if(need_gas_estimation) {
		if(result.status_code != EVMC_SUCCESS) {
			gas_estimated = 0;
		} else {
			gas_estimated = txctrl.estimate_gas(gas_limit);
			if(gas_estimated > 0) { //less than 0 means failing to estimate
				gas_estimated += intrinsic + 5000/*for sstore*/;
			}
		}
	}
	vm->destroy(vm);
	return gas_estimated;
}
