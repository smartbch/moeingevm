#pragma once

#include <ethash/keccak.hpp>
#include "tx_ctrl.h"

const uint64_t MAX_UINT64 = ~uint64_t(0);
const uint64_t MAX_CODE_SIZE = 24576; // 24k

const bool SELFDESTRUCT_BENEFICIARY_CANNOT_BE_PRECOMPILED = false;

extern evmc_bytes32 ZERO_BYTES32;

evmc_address create_contract_addr(const evmc_address& creater, uint64_t nonce);

evmc_address create2_contract_addr(const evmc_address& creater, const evmc_bytes32& salt, const evmc_bytes32& codehash);

// evmc_host_context is an incomplete struct defined in evmc.h, here we make it complete.
// evmone uses this struct to get underlying service
struct evmc_host_context {
private:
	tx_control* txctrl;
	evmc_message msg;
	const bytes* code;
	bytes empty_code;
	small_buffer *smallbuf;
public:
	evmc_host_context(tx_control* tc, evmc_message m, small_buffer* b):
		txctrl(tc), msg(m), empty_code(), smallbuf(b) {}

	static bool is_zero_bytes32(const evmc_bytes32* bytes32) {
		auto ptr = reinterpret_cast<const uint64_t*>(bytes32);
		return (ptr[0]|ptr[1]|ptr[2]|ptr[3]) == 0;
	}

	bool account_exists(const evmc_address& addr) {
		const account_info& info = txctrl->get_account(addr);
		// To pass some of the tests, we need to check bytecode for emptyness, instead of only account sequence
		bool is_empty = (info.nonce == 0 && info.balance == uint256(0) &&
				txctrl->get_bytecode_entry(addr).bytecode.size() == 0);
		//std::cerr<<"@account_exists "<<info.is_null()<<" "<<info.is_empty()<<" "<<info.sequence<<std::endl;
		if(info.is_null() || is_empty) {
			return false;
		}
		return true;
	}
	evmc_bytes32 get_storage(const evmc_address& addr, const evmc_bytes32& key) {
		evmc_bytes32 result;
		const bytes& bz = txctrl->get_value(addr, key);
		if(bz.size() == 0) { // if the underlying KV pair does not exist, return all zero
			return ZERO_BYTES32;
		}
		assert(bz.size() >= 32);
		memcpy(&result.bytes[0], bz.data(), 32);
		return result;
	}
	evmc_storage_status set_storage(const evmc_address& addr, const evmc_bytes32& key, const evmc_bytes32& value) {
		// if the value is zero, set zero-length value to tx_control, which will later be taken as deletion
		size_t size = is_zero_bytes32(&value)? 0 : 32;
		return txctrl->set_value(addr, key, bytes_info{.data=&value.bytes[0], .size=size});
	}
	evmc_uint256be get_balance(const evmc_address& addr) {
		return u256_to_u256be(get_balance_as_uint256(addr));
	}
	uint256 get_balance_as_uint256(const evmc_address& addr) {
		const account_info& info = txctrl->get_account(addr);
		if(info.is_null() || info.is_empty() || info.selfdestructed) {
			return uint256(0);
		}
		return info.balance;
	}
	size_t get_code_size(const evmc_address& addr);
	const evmc_bytes32& get_code_hash(const evmc_address& addr);
	size_t copy_code(const evmc_address& addr, size_t code_offset, uint8_t* buffer_data, size_t buffer_size);
	void selfdestruct(const evmc_address& addr, const evmc_address& beneficiary);
	const evmc_tx_context& get_tx_context() {
		return txctrl->get_tx_context();
	}
	evmc_bytes32 get_block_hash(int64_t height) {
		return txctrl->get_block_hash(height);
	}
	void emit_log(const evmc_address& addr, 
	              const uint8_t* data,
	              size_t data_size,
	              const evmc_bytes32 topics[],
	              size_t topics_count) {
		txctrl->add_log(addr, data, data_size, topics, topics_count);
	}

	void load_code(const evmc_address& addr);
	evmc_result run_precompiled_contract(const evmc_address& addr);
	evmc_result call(const evmc_message* msg);
	evmc_result call();
	evmc_result run_vm(size_t snapshot);
	evmc_result create();
	evmc_result create2();
	bool create_pre_check(const evmc_address& new_addr);
	evmc_result create_with_contract_addr(const evmc_address& addr);
	void check_eip158();

};

