#pragma once

#include <intx/intx.hpp>
#include <string>
#include <vector>
#include <unordered_map>
#include <iostream>
#include <string.h>
#include "bridge.h"
#include "hash.h"

using uint256 = intx::uint256;

using bytes = std::basic_string<uint8_t>;

// EVM only supports 32-byte values, but we can support longer values.
// bytes_info can describe value of abitrary length.
struct bytes_info {
	const uint8_t* data;
	size_t size;
};

// Following are four converting functions for intx::uint256, STL container and evmc_uint256be
inline uint256 bytes_to_u256(const bytes& bz) {
	uint256 result;
	assert(bz.size() == 32);
	memcpy(&result, bz.data(), 32);
	return result;
}

inline bytes u256_to_bytes(const uint256& u) {
	return bytes(reinterpret_cast<const uint8_t*>(&u), 32);
}

inline evmc_uint256be u256_to_u256be(const uint256& u) {
	intx::uint256 v = intx::bswap(u);
	evmc_uint256be result;
	memcpy(&result.bytes[0], &v, 32);
	return result;
}

inline void u256_to_beptr(const uint256& u, uint8_t* ptr) {
	intx::uint256 v = intx::bswap(u);
	memcpy(ptr, &v, 32);
}

inline uint256 u256be_to_u256(const evmc_uint256be& a) {
	return intx::be::load<uint256, 32>(a.bytes);
}

inline uint256 beptr_to_u256(const uint8_t* ptr) {
	evmc_uint256be a;
	memcpy(a.bytes, ptr, 32);
	return u256be_to_u256(a);
}

// Given 0 <= i <= 15, outputs its dex presentation
inline char hex_char(uint8_t i) {
	if(i <= 9) return '0'+i;
	return 'a'+i-10;
}

// We need to show hex presentions of evmc_address, evmc_bytes32 and bytes
inline std::string to_hex(const uint8_t* data, size_t size) {
	std::vector<char> vec(2*size);
	for(size_t i = 0; i < size; i++) {
		vec[2*i] = hex_char((data[i]>>4)&0xF);
		vec[2*i+1] = hex_char(data[i]&0xF);
	}
	return std::string(vec.data(), vec.size());
}

inline std::string to_hex(const evmc_address& addr) {
	return to_hex(&addr.bytes[0], 20);
}

inline std::string to_hex(const evmc_bytes32& data) {
	return to_hex(&data.bytes[0], 32);
}

inline std::string to_hex(const bytes& bz) {
	return to_hex(bz.data(), bz.size());
}

//========================================================================

extern evmc_bytes32 HASH_FOR_ZEROCODE; // keccak hash of a zero-length string

const uint64_t EOA_SEQUENCE = uint64_t(-1);
const uint64_t SEP206_SEQUENCE = uint64_t(2000);

// basic information about an account, NOT including its bytecode
struct account_info {
	bool selfdestructed;
	uint256 balance;
	uint64_t nonce; // If not exists, nonce is uint64_t(-1)
	uint64_t sequence; // For EOA, sequence is uint64_t(-1), For SEP206, uint64_t(-2)
	bool is_null() const {return nonce==uint64_t(-1);}
	bool is_eoa() const {return sequence==EOA_SEQUENCE;}
	bool is_empty() const {return nonce==0 && balance==uint256(0) && is_eoa();}
};

// a cache entry for account
struct account_entry {
	account_info info;
	bool dirty;
};

// a cache entry for smart contracts' creation counter
struct creation_counter_entry {
	uint64_t counter;
	bool dirty;
};

// a cache entry for smart contracts' bytecode
struct bytecode_entry {
	bool deleted;
	bool dirty;
	bytes bytecode;
	evmc_bytes32 codehash;
};

struct evm_log {
	evmc_address contract_addr;
	bytes data;
	std::vector<evmc_bytes32> topics;
	evm_log(const evmc_address& addr, const uint8_t* data_ptr, size_t data_size,
		const evmc_bytes32 topics_ptr[], size_t topics_count):
		contract_addr(addr), data(data_ptr, data_size), topics(topics_count) {
		for(size_t i = 0; i < topics_count; i++) topics[i] = topics_ptr[i];
		//if(topics_count >= 1) {
		//	std::cerr<<"@@@TOPIC "<<to_hex(topics[0])<<std::endl;
		//}
	}
};

//following data structures are used to build a cached world state for running a transaction.
using account_map = std::unordered_map<evmc_address, account_entry, hashfn_evmc_address, equalfn_evmc_address>;
using creation_counter_map = std::unordered_map<uint8_t, creation_counter_entry>;
using bytecode_map = std::unordered_map<evmc_address, bytecode_entry, hashfn_evmc_address, equalfn_evmc_address>;
using value_map = std::unordered_map<storage_key, bytes, hashfn_storage_key, equalfn_storage_key>;

// Read the world state from the underlying Go environment
struct world_state_reader {
	// following function pointers construct a virutal function table
	bridge_get_creation_counter_fn get_creation_counter_fn;
	bridge_get_account_info_fn get_account_info_fn;
	bridge_get_bytecode_fn get_bytecode_fn;
	bridge_get_value_fn get_value_fn;
	bridge_get_block_hash_fn get_block_hash_fn;
	// bigbuf is where Go environment writes bytecode and value.
	big_buffer* bigbuf;
	// the handler to a TxRunner in Go environment
	int handler;
	
	uint64_t get_creation_counter(uint8_t n) {
		return get_creation_counter_fn(handler, n);
	}
	account_info get_account(const evmc_address& addr) {
		evmc_bytes32 balance;
		account_info info = {.selfdestructed=false};
		get_account_info_fn(handler, (evmc_address*)(&addr)/*drop const*/, &balance, &info.nonce, &info.sequence);
		info.balance = u256be_to_u256(balance);
		return info;
	}
	bytes get_bytecode(const evmc_address& addr, evmc_bytes32* codehash) {
		size_t size;
		get_bytecode_fn(handler, (evmc_address*)(&addr)/*drop const*/, codehash, bigbuf, &size);
		return bytes(&bigbuf->data[0], size);
	}
	bytes get_value(uint64_t seq, const evmc_bytes32& key) {
		size_t size;
		get_value_fn(handler, seq, (char*)(&key.bytes[0])/*drop const*/, bigbuf, &size);
		return bytes(&bigbuf->data[0], size);
	}
	evmc_bytes32 get_block_hash(uint64_t num) {
		return get_block_hash_fn(handler, num);
	}
};

// This is a cached subset of the world state, EVM can modify it. And when EVM reverts,
// we undo the modifications to this subset, according to the journal entries.
class cached_state {
	account_map accounts;
	creation_counter_map creation_counters;
	bytecode_map bytecodes;
	value_map values;
	value_map origin_values;
	world_state_reader* world;
	std::vector<evm_log> logs;
	friend struct journal_entry;
	std::vector<internal_tx_call> internal_tx_calls;
	std::vector<internal_tx_return> internal_tx_returns;
	bytes payload_data;
protected:
	//the following protected functions are used by the journal_entry to undo modification
	void _delete_account(const evmc_address& addr) {
		accounts.erase(addr);
	}
	void _decr_nonce(const evmc_address& addr, bool dirty);
	void _set_selfdestructed(const evmc_address& addr, bool b, bool dirty);
	void _incr_balance(const evmc_address& addr, const uint256& amount, bool dirty);
	void _decr_balance(const evmc_address& addr, const uint256& amount, bool dirty);
	void _decr_creation_counter(uint8_t lsb, bool dirty);
	void _set_value(uint64_t sequence, const evmc_bytes32& key, bytes* value);
	void _undelete_bytecode(const evmc_address& addr, bool dirty);
	void _unset_bytecode(const evmc_address& addr, bool dirty);
public:
	uint64_t refund;
	cached_state(world_state_reader* r):
		accounts(), creation_counters(), bytecodes(), values(), world(r), logs(), refund() {
		payload_data.reserve(2048);
	}
	const account_info& get_account(const evmc_address& addr);
	void new_account(const evmc_address& addr);
	void incr_nonce(const evmc_address& addr, bool* old_dirty);
	bool set_selfdestructed(const evmc_address& addr, bool b, bool* old_dirty);
	void incr_balance(const evmc_address& addr, const uint256& amount, bool* old_dirty);
	void decr_balance(const evmc_address& addr, const uint256& amount, bool* old_dirty);
	uint64_t incr_creation_counter(uint8_t lsb, bool* old_dirty);
	const bytes& get_value(uint64_t sequence, const evmc_bytes32& key);
	const bytes& get_origin_value(uint64_t sequence, const evmc_bytes32& key);
	const bytes& set_value(uint64_t sequence, const evmc_bytes32& key, bytes_info value, bytes* old_value);
	const bytecode_entry& get_bytecode_entry(const evmc_address& addr);
	void delete_bytecode(const evmc_address& addr, bool* old_dirty);
	void set_bytecode(const evmc_address& addr, uint64_t sequence, const bytes& code, const evmc_bytes32& codehash, bool* old_dirty);
	void update_bytecode(const evmc_address& addr, const bytes& code, const evmc_bytes32& codehash);
	void add_log(const evmc_address& addr, const uint8_t* data_ptr, size_t data_size,
			const evmc_bytes32 topics_ptr[], size_t topics_count) {
		logs.emplace_back(addr, data_ptr, data_size, topics_ptr, topics_count);
	}
	void pop_log() {
		logs.pop_back();
	}
	bool has_account(const evmc_address& addr) {
		return accounts.find(addr) != accounts.end();
	}
	bool has_value(uint64_t sequence, const evmc_bytes32& key) {
		storage_key k = skey(sequence, key);
		return values.find(k) != values.end();
	}
	// Before the transaction exits, the Go environment should examine how this cached subset was modified.
	// The modified entries in cache are marked as "dirty".
	// These functions collect the modifications and then we pass them to Go by calling collect_result_fn.
	std::vector<changed_account> collect_accounts();
	std::vector<changed_creation_counter> collect_creation_counters();
	std::vector<changed_bytecode> collect_bytecodes();
	std::vector<changed_value> collect_values();
	std::vector<added_log> collect_logs();
	void collect_result(bridge_collect_result_fn collect_result_fn,
			int collector_handler,
			const evmc_result* ret_value);
	void add_internal_tx_call(const evmc_message& msg) {
		internal_tx_call itx_call;
		itx_call.kind = msg.kind;
		itx_call.flags = msg.flags;
		itx_call.depth = msg.depth;
		itx_call.gas = msg.gas;
		itx_call.destination = msg.destination;
		itx_call.sender = msg.sender;
		itx_call.input_offset = payload_data.size();
		itx_call.input_size = msg.input_size;
		itx_call.value = msg.value;
		payload_data.append(msg.input_data, msg.input_size);
		internal_tx_calls.push_back(itx_call);
	}
	void add_internal_tx_return(const evmc_result& res) {
		internal_tx_return itx_return;
		itx_return.status_code = res.status_code;
		itx_return.gas_left = res.gas_left;
		itx_return.output_offset = payload_data.size();
		itx_return.output_size = res.output_size;
		itx_return.create_address = res.create_address;
		payload_data.append(res.output_data, res.output_size);
		internal_tx_returns.push_back(itx_return);
	}
};

// =============================================================

enum journal_type {
	VALUE_CHG = 0,
	ACCOUNT_CREATE,
	BALANCE_CHG,
	NONCE_INCR,
	SELFDESTRUCT_CHG,
	BYTECODE_DEL,
	BYTECODE_CREATE,
	CREATION_COUNTER_INCR,
	REFUND_CHG,
	LOG_QUEUE_ADD,
};

// We use Tagged-Union for journal_entry, instead of interface pointers, because it's friendly 
// to cache and memory allocator.
struct journal_entry {
	journal_type type;
	bytes prev_value; // used by value_change and balance_change
	union {
		struct {
			evmc_bytes32 key;
			uint64_t sequence;
		} value_change;

		struct {
			evmc_address addr;
		} account_creation;

		struct {
			evmc_address sender;
			evmc_address receiver;
			bool sender_old_dirty;
			bool receiver_old_dirty;
			bool is_burn;
		} balance_change;

		struct {
			evmc_address addr;
			bool old_dirty;
		} nonce_incr;

		struct {
			evmc_address addr;
			bool prev_state;
			bool old_dirty;
		} selfdestruct_change;

		struct {
			evmc_address addr;
			bool old_dirty;
		} bytecode_deletion;

		struct {
			evmc_address addr;
			bool old_dirty;
		} bytecode_creation;

		struct {
			uint8_t lsb;
			bool old_dirty;
		} creation_counter_incr;

		struct {
			uint64_t old_refund;
		} refund_change;
	};
	void revert(cached_state* state);
};


// =============================================================

// This class controls the EVMs of a transaction.
class tx_control {
	std::vector<journal_entry> journal;
	std::vector<int64_t> gas_trace; // element with MSB64 means gas-given, otherwise means gas-left
	cached_state cstate;
	world_state_reader* world;
	evmc_tx_context tx_context;
	evmc_execute_fn execute_fn;
	bridge_query_executor_fn query_executor_fn;
	bool need_gas_estimation;
	config cfg;
public:
	// this function provides precompile contracts' functionality from Go to C
	bridge_call_precompiled_contract_fn call_precompiled_contract;

	tx_control(world_state_reader* r, const evmc_tx_context& c, evmc_execute_fn f,
		bridge_query_executor_fn qef, bridge_call_precompiled_contract_fn cpc, bool nge, const config cfg):
		journal(), cstate(r), world(r), tx_context(c), execute_fn(f), query_executor_fn(qef),
		need_gas_estimation(nge), cfg(cfg), call_precompiled_contract(cpc) {
		journal.reserve(100);
		if(need_gas_estimation) {
			gas_trace.reserve(100);
		}
	}

	config get_cfg() {
		return cfg;
	}

	int64_t get_block_number() {
		return tx_context.block_number;
	}

	void gas_trace_append(int64_t gas) {
		if(need_gas_estimation) gas_trace.push_back(gas);
	}
	// Evmone calls this function to execute another smart contract
	evmc_result execute(struct evmc_vm* vm,
	                    const struct evmc_host_interface* host,
	                    struct evmc_host_context* context,
	                    enum evmc_revision rev,
	                    const struct evmc_message* msg,
			    const struct evmc_address* code_addr,
	                    uint8_t const* code,
	                    size_t code_size) {
		evmc_execute_fn executor = nullptr;
		if(query_executor_fn && code_addr) { // Check AOT
			executor = query_executor_fn(code_addr);
		}
		if(!executor) { // fall back to the interpreter
			executor = execute_fn;
		}
		//std::cout<<"query "<<to_hex(msg->destination)<<" "<<size_t(executor)<<std::endl;
		return executor(vm, host, context, rev, msg, code, code_size);
	}
	// a snapshot is just a position of the journal entry list
	size_t snapshot() {
		return journal.size();
	}
	// undo modifications to revert the cached world state to a snapshot
	void revert_to_snapshot(size_t snapshot_id) {
		//std::cerr<<"revert "<<journal.size()<<" => "<<snapshot_id<<std::endl;
		while(journal.size() > snapshot_id) {
			journal.back().revert(&cstate);
			journal.pop_back();
		}
	}
	// append new log to the transaction's log list
	void add_log(const evmc_address& addr, const uint8_t* data_ptr, size_t data_size,
			const evmc_bytes32 topics_ptr[], size_t topics_count) {
		//for(int i = 0; i < topics_count; i++) {
		//	std::cerr<<"@@Topic "<<to_hex(topics_ptr[i])<<std::endl;
		//}
		journal.push_back(journal_entry{.type=LOG_QUEUE_ADD});
		cstate.add_log(addr, data_ptr, data_size, topics_ptr, topics_count);
	}

	// following are some getter&setters for the world state. The modification by setters are recorded in journal
	const account_info& get_account(const evmc_address& addr) {
		return cstate.get_account(addr);
	}
	bool is_selfdestructed(const evmc_address& addr) {
		return cstate.get_account(addr).selfdestructed;
	}
	const uint256& get_balance(const evmc_address& addr) {
		return cstate.get_account(addr).balance;
	}
	void transfer(const evmc_address& sender, const evmc_address& receiver, const uint256& amount);
	void burn(const evmc_address& sender, const uint256& amount);
	void new_account(const evmc_address& addr);
	void incr_nonce(const evmc_address& addr);
	void set_bytecode(const evmc_address& addr, const bytes& code, const evmc_bytes32& codehash);
	void update_bytecode(const evmc_address& addr, const bytes& code, const evmc_bytes32& codehash) {
		cstate.update_bytecode(addr, code, codehash);
	}

	void selfdestruct(const evmc_address& addr);
	evmc_bytes32 get_block_hash(uint64_t height) {
		return world->get_block_hash(height);
	}
	const evmc_tx_context& get_tx_context() {
		return tx_context;
	}
	const bytes& get_value(const evmc_address& addr, const evmc_bytes32& key) {
		const account_info& info = cstate.get_account(addr);
		return cstate.get_value(info.sequence, key);
	}
	const bytes& get_value(uint64_t sequence, const evmc_bytes32& key) {
		return cstate.get_value(sequence, key);
	}
	const bytecode_entry& get_bytecode_entry(const evmc_address& addr) {
		return cstate.get_bytecode_entry(addr);
	}
	enum evmc_access_status access_account(const evmc_address& address) {
		return cstate.has_account(address) ? EVMC_ACCESS_WARM : EVMC_ACCESS_COLD;
	}
	enum evmc_access_status access_storage(const evmc_address& addr, const evmc_bytes32& key) {
		const account_info& info = cstate.get_account(addr);
		return cstate.has_value(info.sequence, key) ? EVMC_ACCESS_WARM : EVMC_ACCESS_COLD;
	}
	evmc_storage_status set_value(const evmc_address& addr, const evmc_bytes32& key, bytes_info value);
	evmc_storage_status set_value(uint64_t sequence, const evmc_bytes32& key, bytes_info value);
	void add_refund(uint64_t delta);
	void sub_refund(uint64_t delta);

	// just forward the function call to underlying cstate
	void add_internal_tx_call(const evmc_message& msg) {
		cstate.add_internal_tx_call(msg);
	}
	void add_internal_tx_return(const evmc_result& res) {
		cstate.add_internal_tx_return(res);
	}
	void collect_result(bridge_collect_result_fn collect_result_fn,
	                    int collector_handler,
	                    const evmc_result* ret_value) {
		cstate.collect_result(collect_result_fn, collector_handler, ret_value);
	}

	int64_t estimate_gas(int64_t init_guess);

};

const uint64_t CALL_NEW_ACCOUNT_GAS = 25000;

const uint64_t SLOAD_GAS = 800;
const uint64_t SSTORE_SET_GAS = 20000;
const uint64_t SSTORE_RESET_GAS = 5000;
const uint64_t SSTORE_CLEARS_SCHEDULE = 15000;
const uint64_t SELFDESTRUCT_REFUND_GAS = 24000;

const uint64_t CREATE_DATA_GAS = 200;
const uint64_t TX_GAS  = 21000; // Per transaction not creating a contract.
const uint64_t TX_GAS_CONTRACT_CREATION = 53000; // Per transaction that creates a contract.
const uint64_t TX_DATA_ZERO_GAS = 4; // Per byte of data attached to a transaction that equals zero.
const uint64_t TX_DATA_NON_ZERO_GAS = 16; // Per byte of data attached to a transaction that is not equal to zero.

const uint64_t MSB64 = (uint64_t(1)<<63);
