#include <evmone/evmone.h>
#include <array>
#include <iostream>
#include "host_context.h"
extern "C" {
#include "../sha256/sha256.h"
#include "../ripemd160/ripemd160.h"
#include "./../evmc/include/evmc/helpers.h"
}

static inline int64_t get_precompiled_id(const evmc_address& addr) {
	for(int i=0; i<12; i++) {
		if(addr.bytes[i] != 0) return -1;
	}
	int64_t res=0;
	for(int i=12; i<20; i++) {
		res <<= 8;
		res |= int64_t(uint64_t(addr.bytes[i]));
	}
	return res;
}

static inline bool is_precompiled(int64_t id) {
	return (1 <= id && id <= 9) || id == SEP101_CONTRACT_ID || id == SEP206_CONTRACT_ID;
}

static inline bool is_precompiled(const evmc_address& addr) {
	return is_precompiled(get_precompiled_id(addr));
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
	.call = evmc_call,
	.get_tx_context = evmc_get_tx_context,
	.get_block_hash = evmc_get_block_hash,
	.emit_log = evmc_emit_log
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
	if(info.sequence == uint64_t(~0)) { // before set_bytecode or EOA
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
		int64_t id = get_precompiled_id(call_msg.destination);
		if(is_precompiled(id)) {
			result = ctx.run_precompiled_contract(call_msg.destination, id);
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

static inline void transfer(tx_control* txctrl, const evmc_address& sender, const evmc_address& destination, const evmc_uint256be& value, bool* is_nop) {
	const account_info& acc = txctrl->get_account(destination);
	bool zero_value = is_zero_bytes32(&value);
	bool call_precompiled = is_precompiled(destination);
	bool is_empty = (acc.nonce == 0 && acc.balance == uint256(0) && 
		txctrl->get_bytecode_entry(destination).bytecode.size() == 0);
	if(acc.is_null() /*&& !call_precompiled*/) {
		if(zero_value && !call_precompiled) {
			*is_nop = true;
			return;
		}
		txctrl->new_account(destination);
	}
	//std::cerr<<"try eip158 "<<is_empty<<" "<<zero_value<<" "<<to_hex(destination)<<std::endl;
	if(is_empty && zero_value) { //eip158
		txctrl->selfdestruct(destination);
	}
	//std::cerr<<"Sender "<<to_hex(sender)<<" Dst "<<to_hex(destination)<<std::endl;
	if(!zero_value /*&& !call_precompiled*/) {
		txctrl->transfer(sender, destination, u256be_to_u256(value));
	}
	*is_nop = false;
}

evmc_result evmc_host_context::call() {
	//std::cerr<<"$msg.gas "<<msg.gas<<std::endl;
	size_t snapshot = txctrl->snapshot();
	load_code(msg.destination);
	bool is_nop;
	transfer(txctrl, msg.sender, msg.destination, msg.value, &is_nop);
	if(is_nop) {
		return evmc_result {.status_code=EVMC_SUCCESS, .gas_left=msg.gas};
	}
	evmc_result result;
	int64_t id = get_precompiled_id(msg.destination);
	if(is_precompiled(id)) {
		result = run_precompiled_contract(msg.destination, id);
		if(result.status_code != EVMC_SUCCESS) {
			txctrl->revert_to_snapshot(snapshot);
		}
	} else {
		result = run_vm(snapshot);
	}
	
	//std::cerr<<" result.gas_left "<<result.gas_left<<std::endl;
	return result;
}

evmc_result evmc_host_context::run_precompiled_contract(const evmc_address& addr, int64_t id) {
	if(id == 2) {
		return run_precompiled_contract_sha256();
	} else if(id == 3) {
		return run_precompiled_contract_ripemd160();
	} else if(id == 4) {
		return run_precompiled_contract_echo();
	} else if(id == SEP101_CONTRACT_ID) {
		return run_precompiled_contract_sep101();
	} else if(id == SEP206_CONTRACT_ID) {
		return run_precompiled_contract_sep206();
	} else if(id == STAKING_CONTRACT_ID) {
		// The staking contract cannot be called by other contracts
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	// the others use golang implementations
	int ret_value, out_of_gas, osize;
	uint64_t gas_left = msg.gas;

	//std::cerr<<"@@inputdata "<<msg.input_size<<" ptr "<<size_t(msg.input_data)<<" ";
	//for(int i = 0; i < msg.input_size; i++) {
	//    std::cerr<<std::hex<<" "<<int(msg.input_data[i]);
	//}
	//std::cerr<<std::dec<<std::endl;

	this->txctrl->call_precompiled_contract((struct evmc_address*)&addr/*drop const*/, (void*)msg.input_data,
			msg.input_size, &gas_left, &ret_value, &out_of_gas, this->smallbuf, &osize);
	if(out_of_gas != 0) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	if(ret_value != 1) {
		return evmc_result{.status_code=EVMC_PRECOMPILED_FAILED};
	}
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(gas_left),
		.output_data=(uint8_t*)&this->smallbuf->data[0],
		.output_size=uint64_t(osize)};
}

inline void sha256(const uint8_t* data, size_t size, uint8_t* out) {
	SHA256_CTX ctx;
	sha256_init(&ctx);
	sha256_update(&ctx, data, size);
	sha256_final(&ctx, out);
}

evmc_result evmc_host_context::run_precompiled_contract_sha256() {
	int64_t gas = (msg.input_size+31)/32*SHA256_PER_WORD_GAS + SHA256_BASE_GAS;
	if(gas > msg.gas) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	sha256(msg.input_data, msg.input_size, (uint8_t*)&this->smallbuf->data[0]);
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas-gas),
		.output_data=(uint8_t*)&this->smallbuf->data[0],
		.output_size=SHA256_BLOCK_SIZE};
}

evmc_result evmc_host_context::run_precompiled_contract_ripemd160() {
	int64_t gas = (msg.input_size+31)/32*RIPEMD160_PER_WORD_GAS + RIPEMD160_BASE_GAS;
	if(gas > msg.gas) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	memset(this->smallbuf->data, 0, 16);
	ripemd160(msg.input_data, msg.input_size, (uint8_t*)&this->smallbuf->data[12]);
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas-gas),
		.output_data=(uint8_t*)&this->smallbuf->data[0],
		.output_size=32};
}

evmc_result evmc_host_context::run_precompiled_contract_echo() {
	int64_t gas = (msg.input_size+31)/32*IDENTITY_PER_WORD_GAS + IDENTITY_BASE_GAS;
	if(gas > msg.gas) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas-gas),
		.output_data=msg.input_data, //forward input to output
		.output_size=msg.input_size};
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
		return evmc_result{.status_code = EVMC_RECREATE_CONTRACT, .gas_left = 0};
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
		int64_t create_data_gas = result.output_size * CREATE_DATA_GAS;
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
		result.status_code = EVMC_EXCEED_MAX_CODE_SIZE;
	}
	result.create_address = addr;
	return result;
}

bool is_zero_address(const evmc_address addr) {
	for(size_t i = 0; i < sizeof(evmc_address); i++) {
		if(addr.bytes[i] != 0) return false;
	}
	return true;
}

// intrinsic gas is the gas consumed before starting EVM
int64_t intrinsic_gas(const uint8_t* input_data, size_t input_size, bool is_contract_creation) {
	int64_t gas = TX_GAS;
	if(is_contract_creation) {
		gas = TX_GAS_CONTRACT_CREATION;
	}
	if(input_size == 0) {
		return gas;
	}
	size_t nz = 0;
	for(size_t i = 0; i < input_size; i++) {
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
	int64_t intrinsic = intrinsic_gas(input_data, input_size, is_contract_creation);
	if(is_contract_creation && intrinsic > gas_limit) {
		// thus we can create zero account (TransactionSendingToZero)
		int64_t no_create_gas = intrinsic_gas(input_data, input_size, false);
		if (no_create_gas <= gas_limit) {
			intrinsic = no_create_gas;
			is_contract_creation = false;
		}
	}
	if(intrinsic > gas_limit) {
		evmc_result result {.status_code=EVMC_OUT_OF_GAS, .gas_left=0};
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
		evmc_result result {.status_code=EVMC_BALANCE_NOT_ENOUGH, .gas_left=msg.gas};
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

// ========================= KV =========================
inline uint32_t get_selector(const uint8_t* data) { //selector is big-endian bytes4
	return  (uint32_t(data[0])<<24)|
		(uint32_t(data[1])<<16)|
		(uint32_t(data[2])<<8)|
		 uint32_t(data[3]);
}

evmc_result evmc_host_context::run_precompiled_contract_sep101() {
	std::cout<<"Dest "<<get_precompiled_id(msg.destination)<<std::endl;
	if(get_precompiled_id(msg.destination) == SEP101_CONTRACT_ID) {// only allow delegatecall
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	if(msg.depth == 0) { // zero-depth-call is forbidden (not accessible from EOA)
		std::cout<<" Fail 0 "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	if(msg.input_size < 4 || msg.input_size > 4 + 32*4 + MAX_KEY_SIZE + MAX_VALUE_SIZE) {
		std::cout<<" Fail 1 "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	uint32_t selector = get_selector(msg.input_data);
	if(selector != SELECTOR_SEP101_SET && selector != SELECTOR_SEP101_GET) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	int offset_ptr_count = (selector == SELECTOR_SEP101_GET)? 1 : 2;
	uint256 key_len_256 = beptr_to_u256(msg.input_data + 4 + offset_ptr_count*32); // skip offset pointers
	if(key_len_256 == 0 || key_len_256 > MAX_KEY_SIZE) {
		std::cout<<" Fail 2 "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	size_t key_len = size_t(key_len_256);
	size_t key_words = (key_len+31)/32;
	if(msg.input_size < 4 + offset_ptr_count*(32/*offset word*/+32/*length word*/) + key_words*32) {
		std::cout<<" Fail 3 "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	evmc_bytes32 key_hash;
	sha256(msg.input_data + 4 + offset_ptr_count*32 + 32/*length word*/, key_len, key_hash.bytes);
	if(selector == SELECTOR_SEP101_GET) {
		std::cout<<" SELECTOR_SEP101_GET "<<std::endl;
		const bytes& bz = txctrl->get_value(msg.destination, key_hash);
		int64_t gas = bz.size() * SEP101_GET_GAS_PER_BYTE;
		if(gas > msg.gas) {
			return evmc_result{.status_code=EVMC_OUT_OF_GAS};
		}
		size_t word_count = 2 + (bz.size()+31)/32;
		uint8_t* buffer = (uint8_t*)malloc(word_count*32);
		memset(buffer, 0, word_count*32);
		buffer[31] = 32; // the offset pointer
		if(bz.size() != 0) {
			u256_to_beptr(uint256(bz.size()), buffer + 32); // length word
			memcpy(buffer + 64, bz.data(), bz.size()); // data payload
		}
		return evmc_result{
			.status_code=EVMC_SUCCESS,
			.gas_left=int64_t(msg.gas-gas),
			.output_data=buffer,
			.output_size=word_count*32,
			.release = evmc_free_result_memory};
	}
	std::cout<<" SELECTOR_SEP101_SET "<<std::endl;
	if((msg.flags & EVMC_STATIC) != 0) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	const uint8_t* value_ptr = msg.input_data + 4 + 3*32 + key_words*32;
	uint256 value_len_256 = beptr_to_u256(value_ptr);
	if(value_len_256 > MAX_VALUE_SIZE) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	size_t value_len = size_t(value_len_256);
	int64_t gas = value_len * SEP101_SET_GAS_PER_BYTE;
	if(gas > msg.gas) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	if(msg.input_size < 4 + 4*32 + key_words*32 + value_len) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	txctrl->set_value(msg.destination, key_hash, bytes_info{.data=value_ptr+32, .size=value_len});

	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas-gas)};
}

// ========================= SEP206 =========================

static evmc_bytes32 ApprovalEvent = {.bytes = {0x25, 0xb9, 0xc3, 0xc7, 0xc8, 0x0a, 0x20, 0x5b, 0x1e, 0x29, 0xb2, 0xf7, 0xc0, 0x14, 0x03, 0xdd, 0xf3, 0x84, 0x1e, 0x7d, 0x42, 0x71, 0x4f, 0xd1, 0x5b, 0x7d, 0xec, 0xeb, 0xe5, 0xe1, 0x5b, 0x8c}};
static evmc_bytes32 TransaferEvent = {.bytes = {0xef, 0xb3, 0x23, 0xf5, 0x4d, 0x5a, 0xf5, 0x28, 0x16, 0xa1, 0xc4, 0x63, 0xf1, 0xa7, 0x2b, 0x95, 0xaa, 0x8d, 0x37, 0xfc, 0x68, 0xb0, 0xc2, 0x69, 0x9b, 0xc8, 0xe2, 0x1b, 0xad, 0x52, 0xf2, 0xdd}};

static inline evmc_result evmc_result_from_str(const std::string& str, uint64_t gas) {
	size_t length = 64 + ((str.size()+31)/32)*32;
	uint8_t* buffer = (uint8_t*)malloc(length);
	memset(buffer, 0, length);
	buffer[31] = 32; // the offset pointer
	buffer[63] = str.size(); // string's size must be less than 255
	memcpy(buffer+64, str.data(), str.size());
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(gas),
		.output_data=buffer,
		.output_size=96,
		.release = evmc_free_result_memory};
}

static inline evmc_result evmc_result_from_uint256(uint256 value, uint64_t gas) {
	uint8_t* buffer = (uint8_t*)malloc(32);
	memset(buffer, 0, 32);
	u256_to_beptr(value, buffer);
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(gas),
		.output_data=buffer,
		.output_size=64,
		.release = evmc_free_result_memory};
}

static inline evmc_result evmc_result_from_bool(bool value, uint64_t gas) {
	uint8_t* buffer = (uint8_t*)malloc(32);
	memset(buffer, 0, 32);
	if(value) {
		buffer[31] = 1;
	}
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(gas),
		.output_data=buffer,
		.output_size=32,
		.release = evmc_free_result_memory};
}

//    function balanceOf(address owner) external view returns (uint);
evmc_result evmc_host_context::sep206_balanceOf() {
	if(msg.input_size != 4 + 32) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	evmc_address addr;
	memcpy(addr.bytes, msg.input_data + 4 + 12, 20);
	evmc_uint256be balance = get_balance(addr);
	uint8_t* buffer = (uint8_t*)malloc(32);
	memcpy(buffer, balance.bytes, 32);
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas),
		.output_data=buffer,
		.output_size=32,
		.release = evmc_free_result_memory};
		
}

//    function allowance(address owner, address spender) external view returns (uint);
evmc_result evmc_host_context::sep206_allowance() {
	if(msg.input_size != 4 + 64) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	evmc_bytes32 key;
	sha256(msg.input_data + 4, 64, key.bytes);
	allowance_entry entry = get_storage_sep206(key);
	assert(memcmp(entry.bytes + 32, ZERO_BYTES32.bytes, 20)==0 ||
	       memcmp(entry.bytes + 32, msg.input_data + 4 + 12, 20)==0);
	assert(memcmp(entry.bytes + 32 + 20, ZERO_BYTES32.bytes, 20)==0 ||
	       memcmp(entry.bytes + 32 + 20, msg.input_data + 4 + 12 + 32, 20)==0);
	uint8_t* buffer = (uint8_t*)malloc(32);
	memcpy(buffer, entry.bytes, 32);
	return evmc_result{
		.status_code=EVMC_SUCCESS,
		.gas_left=int64_t(msg.gas),
		.output_data=buffer,
		.output_size=32,
		.release = evmc_free_result_memory};
}

//    function approve(address spender, uint value) external returns (bool);
evmc_result evmc_host_context::sep206_approve(bool new_value, bool increase) {
	if(msg.input_size != 4 + 64) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	uint8_t owner_and_spender[64]; 
	memset(owner_and_spender, 0, 64);
	memcpy(owner_and_spender + 12, msg.sender.bytes, 20);
	auto spender_offset = msg.input_data + 4 + 12;
	memcpy(owner_and_spender + 32 + 12, spender_offset, 20);
	evmc_address spender;
	memcpy(spender.bytes, spender_offset, 20);
	evmc_bytes32 key;
	sha256(owner_and_spender, 64, key.bytes);
	std::cout<<" approve allowance key "<<to_hex(key)<<" "<<to_hex(msg.sender)<<" "<<to_hex(spender)<<std::endl;
	allowance_entry entry; 
	memcpy(entry.bytes, msg.input_data + 4 + 32, 32);
	memcpy(entry.bytes + 32, msg.sender.bytes, 20);
	memcpy(entry.bytes + 32 + 20, spender_offset, 20);
	if(new_value) {
		set_storage_sep206(key, entry);
	} else {
		allowance_entry old_entry = get_storage_sep206(key);
		assert(memcmp(old_entry.bytes + 32, entry.bytes + 32, 40)==0);
		uint256 allowance_value = beptr_to_u256(old_entry.bytes);
		uint256 delta = beptr_to_u256(entry.bytes);
		if(increase) {
			allowance_value += delta;
			if(allowance_value < delta) { //overflow
				allowance_value = ~uint256(0);
			}
		} else if (allowance_value > delta) {
			allowance_value -= delta;
		} else {
			allowance_value = 0;
		}
		u256_to_beptr(allowance_value, entry.bytes);
		set_storage_sep206(key, entry);
		evmc_bytes32 topics[3];
		memcpy(topics[0].bytes, ApprovalEvent.bytes, 32);
		memset(topics[1].bytes, 0, 16); memcpy(topics[1].bytes + 12, msg.sender.bytes, 20);
		memset(topics[2].bytes, 0, 16); memcpy(topics[2].bytes + 12, spender_offset, 20);
		txctrl->add_log(msg.destination, entry.bytes + 40, 32, topics, 3);
	}
	return evmc_result_from_bool(true, msg.gas);
}

//    function transfer(address to, uint value) external returns (bool);
evmc_result evmc_host_context::sep206_transfer() {
	if(msg.input_size != 4 + 64) {
		std::cout<<" Fail A "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	evmc_address destination;
	memcpy(destination.bytes, msg.input_data + 4 + 12, 20);
	evmc_uint256be amount_be;
	memcpy(amount_be.bytes, msg.input_data + 4 + 32, 32);
	uint256 amount = u256be_to_u256(amount_be);
	evmc_uint256be balance_be = get_balance(msg.sender);
	uint256 balance = u256be_to_u256(balance_be);
	if(balance < amount) {
		return evmc_result{.status_code=EVMC_BALANCE_NOT_ENOUGH};
	}
	bool is_nop;
	transfer(txctrl, msg.sender, destination, amount_be, &is_nop);
	if(!is_nop) {
		evmc_bytes32 topics[3];
		memcpy(topics[0].bytes, TransaferEvent.bytes, 32);
		memset(topics[1].bytes, 0, 16); memcpy(topics[1].bytes + 12, msg.sender.bytes, 20);
		memset(topics[2].bytes, 0, 16); memcpy(topics[2].bytes + 12, destination.bytes, 20);
		txctrl->add_log(msg.destination, amount_be.bytes, 32, topics, 3);
	}
	return evmc_result_from_bool(true, msg.gas);
}

//    function transferFrom(address from, address to, uint value) external returns (bool);
evmc_result evmc_host_context::sep206_transferFrom() {
	if(msg.input_size != 4 + 96) {
		std::cout<<" Fail 0 "<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	evmc_address source, destination;
	memcpy(source.bytes, msg.input_data + 4 + 12, 20);
	memcpy(destination.bytes, msg.input_data + 4 + 32 + 12, 20);
	evmc_uint256be amount_be;
	memcpy(amount_be.bytes, msg.input_data + 4 + 32 + 32, 32);
	uint256 amount = u256be_to_u256(amount_be);
	evmc_uint256be balance_be = get_balance(source);
	uint256 balance = u256be_to_u256(balance_be);
	if(balance < amount) {
		std::cout<<" Fail 1 "<<std::endl;
		return evmc_result{.status_code=EVMC_BALANCE_NOT_ENOUGH};
	}
	uint8_t owner_and_spender[64]; 
	memset(owner_and_spender, 0, 64);
	memcpy(owner_and_spender + 12, source.bytes, 20);
	memcpy(owner_and_spender + 32 + 12, msg.sender.bytes, 20);
	evmc_bytes32 key;
	sha256(owner_and_spender, 64, key.bytes);
	std::cout<<" allowance key "<<to_hex(key)<<" "<<to_hex(source)<<" "<<to_hex(msg.sender)<<std::endl;
	allowance_entry entry = get_storage_sep206(key);
	assert(memcmp(entry.bytes + 32, source.bytes, 20)==0);
	assert(memcmp(entry.bytes + 32 + 20, msg.sender.bytes, 20)==0);
	uint256 allowance_value = beptr_to_u256(entry.bytes);
	if(allowance_value < amount) {
		std::cout<<" Fail 2 "<<int64_t(allowance_value)<<" "<<int64_t(amount)<<std::endl;
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	bool is_nop;
	transfer(txctrl, source, destination, amount_be, &is_nop);
	if(!is_nop) {
		evmc_bytes32 topics[3];
		memcpy(topics[0].bytes, TransaferEvent.bytes, 32);
		memset(topics[1].bytes, 0, 16); memcpy(topics[1].bytes + 12, source.bytes, 20);
		memset(topics[2].bytes, 0, 16); memcpy(topics[2].bytes + 12, destination.bytes, 20);
		txctrl->add_log(msg.destination, amount_be.bytes, 32, topics, 3);
		allowance_value -= amount;
		u256_to_beptr(allowance_value, entry.bytes);
		set_storage_sep206(key, entry);
	}
	return evmc_result_from_bool(true, msg.gas);
}

evmc_result evmc_host_context::run_precompiled_contract_sep206() {
	std::cout<<"Dest "<<get_precompiled_id(msg.destination)<<std::endl;
	if(get_precompiled_id(msg.destination) != SEP206_CONTRACT_ID) {//forbidden delegateccall
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	if(msg.input_size < 4) {
		return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	uint32_t selector = get_selector(msg.input_data);
	uint64_t gas;
	switch(selector) {
		case SELECTOR_SEP206_NAME:
			gas = SEP206_NAME_GAS;
		break;
		case SELECTOR_SEP206_SYMBOL:
			gas = SEP206_SYMBOL_GAS;
		break;
		case SELECTOR_SEP206_DECIMALS:
			gas = SEP206_DECIMALS_GAS;
		break;
		case SELECTOR_SEP206_TOTALSUPPLY:
			gas = SEP206_TOTALSUPPLY_GAS;
		break;
		case SELECTOR_SEP206_BALANCEOF:
			gas = SEP206_BALANCEOF_GAS;
		break;
		case SELECTOR_SEP206_ALLOWANCE:
			gas = SEP206_ALLOWANCE_GAS;
		break;
		case SELECTOR_SEP206_APPROVE:
			gas = SEP206_APPROVE_GAS;
		break;
		case SELECTOR_SEP206_INCREASEALLOWANCE:
			gas = SEP206_INCREASEALLOWANCE_GAS;
		break;
		case SELECTOR_SEP206_DECREASEALLOWANCE:
			gas = SEP206_DECREASEALLOWANCE_GAS;
		break;
		case SELECTOR_SEP206_TRANSFER:
			gas = SEP206_TRANSFER_GAS;
		break;
		case SELECTOR_SEP206_TRANSFERFROM:
			gas = SEP206_TRANSFERFROM_GAS;
		break;
		default:
			return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
	}
	if(gas > msg.gas) {
		return evmc_result{.status_code=EVMC_OUT_OF_GAS};
	}
	msg.gas -= gas;
	switch(selector) { // staticcall must be readonly
		case SELECTOR_SEP206_APPROVE:
		case SELECTOR_SEP206_INCREASEALLOWANCE:
		case SELECTOR_SEP206_DECREASEALLOWANCE:
		case SELECTOR_SEP206_TRANSFER:
		case SELECTOR_SEP206_TRANSFERFROM:
			if((msg.flags & EVMC_STATIC) != 0) {
				return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
			}
			break;
		default:
			break;
	}
	switch(selector) {
		case SELECTOR_SEP206_NAME:
			return evmc_result_from_str("BCH", msg.gas);
		case SELECTOR_SEP206_SYMBOL:
			return evmc_result_from_str("BCH", msg.gas);
		case SELECTOR_SEP206_DECIMALS:
			return evmc_result_from_uint256(uint256(18), msg.gas);
		case SELECTOR_SEP206_TOTALSUPPLY:
			return evmc_result_from_uint256(uint256(2100*10000)*uint256(1000000000000000000), msg.gas);
		case SELECTOR_SEP206_BALANCEOF:
			return sep206_balanceOf();
		case SELECTOR_SEP206_ALLOWANCE:
			return sep206_allowance();
		case SELECTOR_SEP206_APPROVE:
			return sep206_approve(true, false);
		case SELECTOR_SEP206_INCREASEALLOWANCE:
			return sep206_approve(false, true);
		case SELECTOR_SEP206_DECREASEALLOWANCE:
			return sep206_approve(false, false);
		case SELECTOR_SEP206_TRANSFER:
			return sep206_transfer();
		case SELECTOR_SEP206_TRANSFERFROM:
			return sep206_transferFrom();
	}
	return evmc_result{.status_code=EVMC_PRECOMPILE_FAILURE};
}

