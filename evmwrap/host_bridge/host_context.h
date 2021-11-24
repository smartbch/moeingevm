#pragma once

#include <ethash/keccak.hpp>
#include "tx_ctrl.h"

const int64_t HARD_FORK_HEIGH_1 = 2801234;

const uint64_t MAX_UINT64 = ~uint64_t(0);
const uint64_t MAX_CODE_SIZE = 24576; // 24k
const uint64_t MAX_KEY_SIZE = 256;
const uint64_t MAX_VALUE_SIZE = 24576; // 24k

const uint32_t SELECTOR_SEP101_GET = 0xd6d7d525;
const uint32_t SELECTOR_SEP101_SET = 0xa18c751e;
const uint32_t SELECTOR_SEP206_NAME = 0x06fdde03;
const uint32_t SELECTOR_SEP206_SYMBOL = 0x95d89b41;
const uint32_t SELECTOR_SEP206_DECIMALS = 0x313ce567;
const uint32_t SELECTOR_SEP206_TOTALSUPPLY = 0x18160ddd;
const uint32_t SELECTOR_SEP206_BALANCEOF = 0x70a08231;
const uint32_t SELECTOR_SEP206_ALLOWANCE = 0xdd62ed3e;
const uint32_t SELECTOR_SEP206_APPROVE = 0x095ea7b3;
const uint32_t SELECTOR_SEP206_INCREASEALLOWANCE = 0x39509351;
const uint32_t SELECTOR_SEP206_DECREASEALLOWANCE = 0xa457c2d7;
const uint32_t SELECTOR_SEP206_TRANSFER = 0xa9059cbb;
const uint32_t SELECTOR_SEP206_TRANSFERFROM = 0x23b872dd;

const uint64_t SHA256_BASE_GAS = 60;
const uint64_t SHA256_PER_WORD_GAS = 12;
const uint64_t RIPEMD160_BASE_GAS = 600;
const uint64_t RIPEMD160_PER_WORD_GAS = 120;
const uint64_t IDENTITY_BASE_GAS = 15;
const uint64_t IDENTITY_PER_WORD_GAS = 3;
const uint64_t SEP101_GET_GAS_PER_BYTE = 25;
const uint64_t SEP101_SET_GAS_PER_BYTE = 75;
const uint32_t SEP206_NAME_GAS = 3000;
const uint32_t SEP206_SYMBOL_GAS = 3000;
const uint32_t SEP206_DECIMALS_GAS = 1000;
const uint32_t SEP206_TOTALSUPPLY_GAS = 1000;
const uint32_t SEP206_BALANCEOF_GAS = 20000;
const uint32_t SEP206_ALLOWANCE_GAS = 20000;
const uint32_t SEP206_APPROVE_GAS = 25000;
const uint32_t SEP206_INCREASEALLOWANCE_GAS = 31000;
const uint32_t SEP206_DECREASEALLOWANCE_GAS = 31000;
const uint32_t SEP206_TRANSFER_GAS = 32000;
const uint32_t SEP206_TRANSFERFROM_GAS = 40000;

const int64_t SEP101_CONTRACT_ID = 0x2712;
const int64_t SEP206_CONTRACT_ID = 0x2711;
const int64_t STAKING_CONTRACT_ID = 0x2710;


const bool SELFDESTRUCT_BENEFICIARY_CANNOT_BE_PRECOMPILED = false;

extern evmc_bytes32 ZERO_BYTES32;

evmc_address create_contract_addr(const evmc_address& creater, uint64_t nonce);

evmc_address create2_contract_addr(const evmc_address& creater, const evmc_bytes32& salt, const evmc_bytes32& codehash);

static inline bool is_zero_bytes32(const uint8_t* u8ptr) {
	auto ptr = reinterpret_cast<const uint64_t*>(u8ptr);
	return (ptr[0]|ptr[1]|ptr[2]|ptr[3]) == 0;
}

static inline bool is_zero_address(const evmc_address addr) {
	for(size_t i = 0; i < sizeof(evmc_address); i++) {
		if(addr.bytes[i] != 0) return false;
	}
	return true;
}

const int ALLOWANCE_ENTRY_SIZE = 32+20+20;
struct allowance_entry {
	uint8_t bytes[ALLOWANCE_ENTRY_SIZE];
};

// evmc_host_context is an incomplete struct defined in evmc.h, here we make it complete.
// evmone uses this struct to get underlying service
struct evmc_host_context {
private:
	tx_control* txctrl;
	evmc_message msg;
	const bytes* code;
	bytes empty_code;
	small_buffer *smallbuf;
	enum evmc_revision revision;
public:
	evmc_host_context(tx_control* tc, evmc_message m, small_buffer* b, enum evmc_revision r):
		txctrl(tc), msg(m), empty_code(), smallbuf(b), revision(r) {}

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
	allowance_entry get_storage_sep206(const evmc_bytes32& key) {
		allowance_entry result={};
		const bytes& bz = txctrl->get_value(SEP206_SEQUENCE, key);
		if(bz.size() == 0) { // if the underlying KV pair does not exist, return all zero
			return result;
		}
		assert(bz.size() >= ALLOWANCE_ENTRY_SIZE);
		memcpy(result.bytes, bz.data(), ALLOWANCE_ENTRY_SIZE);
		return result;
	}
	evmc_storage_status set_storage(const evmc_address& addr, const evmc_bytes32& key, const evmc_bytes32& value) {
		// if the value is zero, set zero-length value to tx_control, which will later be taken as deletion
		size_t size = is_zero_bytes32(value.bytes)? 0 : 32;
		return txctrl->set_value(addr, key, bytes_info{.data=&value.bytes[0], .size=size});
	}
	void set_storage_sep206(const evmc_bytes32& key, const allowance_entry& value) {
		// if the value is zero, set zero-length value to tx_control, which will later be taken as deletion
		size_t size = is_zero_bytes32(value.bytes)? 0 : ALLOWANCE_ENTRY_SIZE;
		txctrl->set_value(SEP206_SEQUENCE, key, bytes_info{.data=value.bytes, .size=size});
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
	evmc_result run_precompiled_contract(const evmc_address& addr, int64_t id);
	evmc_result run_precompiled_contract_sha256();
	evmc_result run_precompiled_contract_ripemd160();
	evmc_result run_precompiled_contract_echo();
	evmc_result run_precompiled_contract_sep101();
	evmc_result run_precompiled_contract_sep206();
	evmc_result sep206_balanceOf();
	evmc_result sep206_allowance();
	evmc_result sep206_approve(bool new_value, bool increase);
	evmc_result sep206_transfer();
	evmc_result sep206_transferFrom();
	evmc_result call(const evmc_message& msg);
	evmc_result call();
	evmc_result run_vm(size_t snapshot);
	evmc_result create();
	evmc_result create2();
	bool create_pre_check(const evmc_address& new_addr);
	evmc_result create_with_contract_addr(const evmc_address& addr);
	enum evmc_access_status access_account(const evmc_address& address) {
		return txctrl->access_account(address);
	}
	enum evmc_access_status access_storage(const evmc_address& addr, const evmc_bytes32& key) {
		return txctrl->access_storage(addr, key);
	}
	void check_eip158();
};
