#include "tx_ctrl.h"

evmc_bytes32 HASH_FOR_ZEROCODE {
	.bytes = {0xc5, 0xd2, 0x46, 0x01, 0x86, 0xf7, 0x23, 0x3c,
	          0x92, 0x7e, 0x7d, 0xb2, 0xdc, 0xc7, 0x03, 0xc0,
	          0xe5, 0x00, 0xb6, 0x53, 0xca, 0x82, 0x27, 0x3b,
	          0x7b, 0xfa, 0xd8, 0x04, 0x5d, 0x85, 0xa4, 0x70}
};

// get account info from cache, or from world state and insert it to cache
const account_info& cached_state::get_account(const evmc_address& addr) {
	auto iter = accounts.find(addr);
	if(iter != accounts.end()) {
		return iter->second.info;
	}
	account_entry entry {};
	entry.info = world->get_account(addr);
	entry.info.selfdestructed = false;
	entry.dirty = false;
	accounts[addr] = entry;
	return accounts[addr].info;
}

// create a new account and insert it into cache
void cached_state::new_account(const evmc_address& addr) {
	account_entry entry {};
	entry.info.balance = uint256(0);
	entry.info.nonce = 0;
	entry.info.sequence = ~0;
	entry.info.selfdestructed = false;
	entry.dirty = true;
	accounts[addr] = entry;
}

// increase the nonce of a cached account. EVM must have read an account before increasing its nonce
void cached_state::incr_nonce(const evmc_address& addr, bool* old_dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	iter->second.info.nonce++;
	//std::cerr<<"@@incr_nonce "<<to_hex(addr)<<" "<<iter->second.info.nonce<<std::endl;
	*old_dirty = iter->second.dirty;
	iter->second.dirty = true;
}

// undo the effects of incr_nonce
void cached_state::_decr_nonce(const evmc_address& addr, bool dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	iter->second.info.nonce--;
	iter->second.dirty = dirty;
}

// mark an account as selfdestructed, a selfdestructed account can be selfdestructed again
bool cached_state::set_selfdestructed(const evmc_address& addr, bool b, bool* old_dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	bool old_value = iter->second.info.selfdestructed;
	*old_dirty = iter->second.dirty;
	iter->second.dirty = true;
	iter->second.info.selfdestructed = b;
	return old_value;
}

// undo the effects of set_selfdestructed
void cached_state::_set_selfdestructed(const evmc_address& addr, bool b, bool dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	iter->second.info.selfdestructed = b;
	iter->second.dirty = dirty;
}

// increase an account's balance
bool cached_state::incr_balance(const evmc_address& addr, const uint256& amount, bool* old_dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	auto new_balance = iter->second.info.balance + amount;
	if(new_balance < iter->second.info.balance) {
		return false;
	}
	iter->second.info.balance = new_balance;
	*old_dirty = iter->second.dirty;
	iter->second.dirty = true;
	return true;
}

// undo the effects of decr_balance
void cached_state::_incr_balance(const evmc_address& addr, const uint256& amount, bool dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	iter->second.info.balance += amount;
	iter->second.dirty = dirty;
}

// increase an account's balance
bool cached_state::decr_balance(const evmc_address& addr, const uint256& amount, bool* old_dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	if(iter->second.info.balance < amount) {
		return false;
	}
	*old_dirty = iter->second.dirty;
	iter->second.dirty = true;
	iter->second.info.balance -= amount;
	return true;
}

// undo the effects of incr_balance
void cached_state::_decr_balance(const evmc_address& addr, const uint256& amount, bool dirty) {
	auto iter = accounts.find(addr);
	assert(iter != accounts.end());
	iter->second.info.balance -= amount;
	iter->second.dirty = dirty;
}

// increase a cached creation counter, or fetch the counter from world state and increase it
uint64_t cached_state::incr_creation_counter(uint8_t lsb, bool* old_dirty) {
	auto iter = creation_counters.find(lsb);
	if(iter != creation_counters.end()) {
		iter->second.counter++;
		*old_dirty = iter->second.dirty;
		iter->second.dirty = true;
		return iter->second.counter;
	}
	uint64_t counter = world->get_creation_counter(lsb) + 1;
	*old_dirty = false;
	creation_counters[lsb] = creation_counter_entry{.counter=counter, .dirty=true};
	return counter;
}

// undo the effects of incr_creation_counter
void cached_state::_decr_creation_counter(uint8_t lsb, bool dirty) {
	creation_counters[lsb].counter--;
	creation_counters[lsb].dirty = dirty;
}

// get bytecode&codehash from cache, or from world state and insert it to cache
const bytecode_entry& cached_state::get_bytecode_entry(const evmc_address& addr) {
	auto iter = bytecodes.find(addr);
	if(iter != bytecodes.end()) {
		return iter->second;
	}
	bytecode_entry e {.deleted=false, .dirty=false};
	e.bytecode = world->get_bytecode(addr, &e.codehash);
	if(e.bytecode.size() == 0) {
		e.codehash = HASH_FOR_ZEROCODE;
	}
	bytecodes[addr] = e;
	return bytecodes[addr];
}

// delete the stored bytecode when a contract is selfdestructed
void cached_state::delete_bytecode(const evmc_address& addr, bool* old_dirty) {
	*old_dirty = bytecodes[addr].dirty;
	bytecodes[addr].dirty = true;
	bytecodes[addr].deleted = true;
}

// undo the effect of delete_bytecode
void cached_state::_undelete_bytecode(const evmc_address& addr, bool dirty) {
	bytecodes[addr].dirty = dirty;
	bytecodes[addr].deleted = false;
}

// create bytecode for a new contract
void cached_state::set_bytecode(const evmc_address& addr, uint64_t sequence, const bytes& code, const evmc_bytes32& codehash, bool* old_dirty) {
	auto iter = bytecodes.find(addr);
	if(iter == bytecodes.end()) {
		*old_dirty = false;
	} else {
		*old_dirty = iter->second.dirty;
	}
	bytecodes[addr] = bytecode_entry{.deleted=false, .dirty=true, .bytecode=code, .codehash=codehash};

	accounts[addr].info.sequence = sequence;
}

// update the bytecode for a new contract, please note a 'set_bytecode' is alwasys followed by a 'update_bytecode'.
// They are atomic: both revert or both take effect.
void cached_state::update_bytecode(const evmc_address& addr, const bytes& code, const evmc_bytes32& codehash) {
	bytecodes[addr].bytecode = code;
	bytecodes[addr].codehash = codehash;
}

// undo the effect of set_bytecode and the following update_bytecode
void cached_state::_unset_bytecode(const evmc_address& addr, bool dirty) {
	bytecodes[addr].deleted = true;
	bytecodes[addr].dirty = dirty;
	accounts[addr].info.sequence = ~0;
}

// get bytecode&codehash from cache, or from world state and insert it to value cache & original value cache
const bytes& cached_state::get_value(uint64_t sequence, const evmc_bytes32& key) {
	storage_key k = skey(sequence, key);
	auto iter = values.find(k);
	if(iter != values.end()) {
		return iter->second;
	}
	bytes value = world->get_value(sequence, key);
	origin_values[k] = value;
	values[k] = value;
	return values[k];
}

// get the original value, that is, the unmodified version from underlying world state.
// we need the original value to calculate gas fee of SSTORE
const bytes& cached_state::get_origin_value(uint64_t sequence, const evmc_bytes32& key) {
	auto iter = origin_values.find(skey(sequence, key));
	assert(iter != origin_values.end());
	return iter->second;
}

// try to modify a value in cache. If cache misses, fetch the value from world state and insert
// it to value cache & original value cache
const bytes& cached_state::set_value(uint64_t sequence, const evmc_bytes32& key, bytes_info value, bytes* old_value) {
	storage_key k = skey(sequence, key);
	auto iter = values.find(k);
	if(iter == values.end()) {
		bytes value = world->get_value(sequence, key);
		values[k] = value;
		origin_values[k] = value;
		iter = values.find(k);
	}
	*old_value = std::move(iter->second);
	iter->second = bytes(value.data, value.size);
	return iter->second;
}

// undo the effect of set_value
void cached_state::_set_value(uint64_t sequence, const evmc_bytes32& key, bytes* value) {
	values[skey(sequence, key)] = std::move(*value);
}

// following functions collect the modifed(dirty) entries from several caches, which will
// be passed to Go environment
std::vector<changed_account> cached_state::collect_accounts() {
	std::vector<changed_account> result;
	result.reserve(accounts.size());
	for(auto& [addr, acc] : accounts) {
		//std::cerr<<"@ChangeAccount "<<std::hex<<size_t(&addr)<<" "<<std::dec<<to_hex(addr)<<" nonce "<<acc.info.nonce<<" b "<<intx::to_string(acc.info.balance)<<" seq "<<acc.info.sequence<<" dirty "<<acc.dirty<<" selfdestructed "<<acc.info.selfdestructed<<" null "<<acc.info.is_null()<<std::endl;
		if(!acc.dirty) continue;
		if(acc.info.is_null()) continue;
		if(acc.info.is_empty() && !acc.info.selfdestructed) continue;
		//std::cerr<<"!ChangeAccount "<<to_hex(addr)<<" nonce "<<acc.info.nonce<<" b "<<intx::to_string(acc.info.balance)<<" seq "<<acc.info.sequence<<std::endl;
		result.push_back(changed_account{
			.address = &addr,
			.balance = u256_to_u256be(acc.info.balance),
			.nonce = acc.info.nonce,
			.sequence = acc.info.sequence,
			.delete_me = acc.info.selfdestructed
		});
	}
	return result;
}

std::vector<changed_creation_counter> cached_state::collect_creation_counters() {
	std::vector<changed_creation_counter> result;
	result.reserve(creation_counters.size());
	for(auto elem : creation_counters) {
		if(!elem.second.dirty) continue;
		result.push_back(changed_creation_counter{
			.lsb = elem.first,
			.counter = elem.second.counter
		});
	}
	return result;
}

std::vector<changed_bytecode> cached_state::collect_bytecodes() {
	std::vector<changed_bytecode> result;
	result.reserve(bytecodes.size());
	for(auto& [key, value] : bytecodes) {
		if(!value.dirty) continue;
		result.push_back(changed_bytecode{
			.address = &key,
			.bytecode_data = (char*)value.bytecode.data(),
			.bytecode_size = value.deleted? 0 : int(value.bytecode.size()),
			.codehash = &value.codehash
		});
	}
	return result;
}

std::vector<changed_value> cached_state::collect_values() {
	std::vector<changed_value> result;
	result.reserve(values.size());
	for(auto& [key, value] : values) {
		if(value == origin_values[key]) continue;
		//std::cerr<<"!ChangeValue key "<<to_hex(key.key)<<" value ";
		//for(int i  = 0; i < value.size(); i++) {
		//	std::cerr<<" "<<int(value[i]);
		//}
		//std::cerr<<std::endl;
		result.push_back(changed_value{
			.account_seq = key.account_seq,
			.key_ptr = (char*)&key.key.bytes[0],
			.value_data = (char*)value.data(),
			.value_size = int(value.size())
		});
	}
	return result;
}

std::vector<added_log> cached_state::collect_logs() {
	std::vector<added_log> result;
	result.reserve(logs.size());
	for(size_t i = 0; i < logs.size(); i++) {
		evm_log& elem = logs[i];
		added_log log {
			.contract_addr = &elem.contract_addr,
			.data = (char*)elem.data.data(),
			.size = int(elem.data.size()),
			.topic1 = nullptr,
			.topic2 = nullptr,
			.topic3 = nullptr,
			.topic4 = nullptr
		};
		if(elem.topics.size() >= 1) log.topic1 = &elem.topics[0];
		if(elem.topics.size() >= 2) log.topic2 = &elem.topics[1];
		if(elem.topics.size() >= 3) log.topic3 = &elem.topics[2];
		if(elem.topics.size() >= 4) log.topic4 = &elem.topics[3];
		result.push_back(log);
	}
	return result;
}

void cached_state::collect_result(bridge_collect_result_fn collect_result_fn,
                                  int collector_handler,
                                  const evmc_result* ret_value) {
	std::vector<changed_account> changed_accounts = collect_accounts();
	std::vector<changed_creation_counter> changed_creation_counters = collect_creation_counters();
	std::vector<changed_bytecode> changed_bytecodes = collect_bytecodes();
	std::vector<changed_value> changed_values = collect_values();
	std::vector<added_log> added_logs = collect_logs();
	all_changed changes {
		.accounts = changed_accounts.data(),
		.account_num = changed_accounts.size(),
		.creation_counters = changed_creation_counters.data(),
		.creation_counter_num = changed_creation_counters.size(),
		.bytecodes = changed_bytecodes.data(),
		.bytecode_num = changed_bytecodes.size(),
		.values = changed_values.data(),
		.value_num = changed_values.size(),
		.logs = added_logs.data(),
		.log_num = added_logs.size(),
		.refund = refund,
		.data_ptr = reinterpret_cast<std::uintptr_t>(payload_data.data()),
		.internal_tx_calls = internal_tx_calls.data(),
		.internal_tx_call_num = internal_tx_calls.size(),
		.internal_tx_returns = internal_tx_returns.data(),
		.internal_tx_return_num = internal_tx_returns.size()
	};
	//std::cerr<<"Here in collect_result "<<size_t(&changes)<<std::endl;
	// use the following callback function to pass changes to Go environment
	collect_result_fn(collector_handler, &changes, (evmc_result*)ret_value/*drop const*/);
}

// ============================================================

// revert one modification to the cached state, according to one journal entry
void journal_entry::revert(cached_state* state) {
	uint256 amount;
	switch(this->type) { // perform different actions depending on the union's tag
	case VALUE_CHG:
		state->_set_value(value_change.sequence, value_change.key, &this->prev_value);
		break;
	case ACCOUNT_CREATE:
		state->_delete_account(account_creation.addr);
		break;
	case BALANCE_CHG:
		amount = bytes_to_u256(this->prev_value);
		state->_incr_balance(balance_change.sender, amount, balance_change.sender_old_dirty);
		if (!balance_change.is_burn) {
			state->_decr_balance(balance_change.receiver, amount, balance_change.receiver_old_dirty);
		}
		break;
	case NONCE_INCR:
		state->_decr_nonce(nonce_incr.addr, nonce_incr.old_dirty);
		break;
	case SELFDESTRUCT_CHG:
		state->_set_selfdestructed(selfdestruct_change.addr, selfdestruct_change.prev_state,
				selfdestruct_change.old_dirty);
		break;
	case BYTECODE_DEL:
		state->_undelete_bytecode(bytecode_deletion.addr, bytecode_deletion.old_dirty);
		break;
	case BYTECODE_CREATE:
		state->_unset_bytecode(bytecode_creation.addr, bytecode_creation.old_dirty);
		break;
	case CREATION_COUNTER_INCR:
		state->_decr_creation_counter(creation_counter_incr.lsb, creation_counter_incr.old_dirty);
		break;
	case REFUND_CHG:
		state->refund = refund_change.old_refund;
		break;
	case LOG_QUEUE_ADD:
		state->pop_log();
		break;
	}
}

// =============================================================

// tx_control perform high-level operations on the cached state and record journals of these operations, because
// these operations may be reverted later.

bool tx_control::transfer(const evmc_address& sender, const evmc_address& receiver, const uint256& amount) {
	journal_entry e {.type=BALANCE_CHG};
	e.prev_value = u256_to_bytes(amount);
	e.balance_change.sender = sender;
	e.balance_change.receiver = receiver;
	e.balance_change.is_burn = false;
	if(!cstate.decr_balance(sender, amount, &e.balance_change.sender_old_dirty)) {
		return false;
	}
	if(!cstate.incr_balance(receiver, amount, &e.balance_change.receiver_old_dirty)) {
		return false;
	}
	journal.push_back(e);
	return true;
}

void tx_control::burn(const evmc_address& sender, const uint256& amount) {
	journal_entry e {.type=BALANCE_CHG};
	e.prev_value = u256_to_bytes(amount);
	e.balance_change.sender = sender;
	e.balance_change.is_burn = true;
	cstate.decr_balance(sender, amount, &e.balance_change.sender_old_dirty);
	journal.push_back(e);
}

void tx_control::new_account(const evmc_address& addr) {
	journal_entry e {.type=ACCOUNT_CREATE};
	e.account_creation.addr = addr;
	cstate.new_account(addr);
	journal.push_back(e);
}

void tx_control::incr_nonce(const evmc_address& addr) {
	journal_entry e {.type=NONCE_INCR};
	e.nonce_incr.addr = addr;
	cstate.incr_nonce(addr, &e.nonce_incr.old_dirty);
	journal.push_back(e);
}

void tx_control::set_bytecode(const evmc_address& addr, const bytes& code, const evmc_bytes32& codehash) {
	// when we create a contract account with 'set_bytecode', we must assign a new sequence to it
	journal_entry e {.type=CREATION_COUNTER_INCR};
	e.creation_counter_incr.lsb = addr.bytes[0];
	uint64_t counter = cstate.incr_creation_counter(addr.bytes[0], &e.creation_counter_incr.old_dirty);
	journal.push_back(e);
	uint64_t sequence = (counter<<8) | uint64_t(addr.bytes[0]);

	e = journal_entry{.type=BYTECODE_CREATE};
	e.bytecode_creation.addr = addr;
	cstate.set_bytecode(addr, sequence, code, codehash, &e.bytecode_creation.old_dirty);
	journal.push_back(e);
}

void tx_control::selfdestruct(const evmc_address& addr) {
	const account_info& acc_info = cstate.get_account(addr);
	if(acc_info.selfdestructed) { // During a TX, a contract can be selfdestructed many times.
		return;
	}
	// for selfdestructed, the account and the bytecode must be deleted seperately, because
	// they are stored seperately.
	journal_entry e {.type=SELFDESTRUCT_CHG};
	e.selfdestruct_change.addr = addr;
	e.selfdestruct_change.prev_state = cstate.set_selfdestructed(addr, true, &e.selfdestruct_change.old_dirty);
	journal.push_back(e);

	e = journal_entry{.type=BYTECODE_DEL};
	e.bytecode_deletion.addr = addr;
	cstate.delete_bytecode(addr, &e.bytecode_deletion.old_dirty);
	journal.push_back(e);
}

// for SSTORE's gas&refund calculation, we must return correct evmc_storage_status according to EIP-2200
evmc_storage_status tx_control::set_value(const evmc_address& addr, const evmc_bytes32& key, bytes_info raw_value) {
	const account_info& info = cstate.get_account(addr);
	return set_value(info.sequence, key, raw_value);
}

evmc_storage_status tx_control::set_value(uint64_t sequence, const evmc_bytes32& key, bytes_info raw_value) {
	journal_entry e {.type=VALUE_CHG};
	e.value_change.key = key;
	e.value_change.sequence = sequence;
	const bytes& new_value = cstate.set_value(sequence, key, raw_value, &e.prev_value);
	journal.push_back(e);
	const bytes& origin = cstate.get_origin_value(sequence, key);
	//If current value equals new value (this is a no-op), SLOAD_GAS is deducted.
	if(e.prev_value == new_value) {
		return EVMC_STORAGE_UNCHANGED;
	}
	//If current value does not equal new value:
	//If original value equals current value, that is, 
	//this storage slot has not been changed by the current execution context
	if(origin == e.prev_value) {
		//If original value is 0, SSTORE_SET_GAS is deducted.
		if(origin.size() == 0) {
			return EVMC_STORAGE_ADDED;
		} else {
		//Otherwise, SSTORE_RESET_GAS gas is deducted. If new value is 0, add SSTORE_CLEARS_SCHEDULE
		//gas to refund counter.
			if(new_value.size() == 0) {
				add_refund(SSTORE_CLEARS_SCHEDULE);
				return EVMC_STORAGE_DELETED;
			} else {
				return EVMC_STORAGE_MODIFIED;
			}
		}
	//If original value does not equal current value (this storage slot is dirty), SLOAD_GAS gas is deducted.
	} else {
		//If original value is not 0
		if(origin.size() != 0) {
			//If current value is 0 (also means that new value is not 0), remove SSTORE_CLEARS_SCHEDULE
			//gas from refund counter.
			if(e.prev_value.size() == 0) {
				sub_refund(SSTORE_CLEARS_SCHEDULE);
			}
			//If new value is 0 (also means that current value is not 0), add SSTORE_CLEARS_SCHEDULE
			//gas to refund counter.
			if(new_value.size() == 0) {
				add_refund(SSTORE_CLEARS_SCHEDULE);
			}
		}
		//If original value equals new value (this storage slot is reset)
		if(origin == new_value) {
			//If original value is 0, add SSTORE_SET_GAS - SLOAD_GAS to refund counter.
			if(origin.size() == 0) {
				add_refund(SSTORE_SET_GAS - SLOAD_GAS);
			} else {
			//Otherwise, add SSTORE_RESET_GAS - SLOAD_GAS gas to refund counter.
				add_refund(SSTORE_RESET_GAS - SLOAD_GAS);
			}
		}
		return EVMC_STORAGE_MODIFIED_AGAIN;
	}
}

void tx_control::add_refund(uint64_t delta) {
	journal_entry e {.type=REFUND_CHG};
	e.refund_change.old_refund = cstate.refund;
	cstate.refund += delta;
	journal.push_back(e);
}

void tx_control::sub_refund(uint64_t delta) {
	journal_entry e {.type=REFUND_CHG};
	e.refund_change.old_refund = cstate.refund;
	cstate.refund -= delta;
	journal.push_back(e);
}

// ==============================================

const int64_t ERROR = -100;
const int64_t CALL_EVM = -1;
const int64_t THRES = 50;

// use data in gas_trace to fill gas_consumed.
// in gas_consumed, a CALL_EVM element means calling a contract, otherwise
// the element equaling N means a contract returns with N gas used.
int64_t fill_gas_consumed(std::vector<int64_t>& gas_consumed, const std::vector<int64_t>& gas_trace, int& offset, int64_t gas_given) {
	int64_t gas_used_by_sub = 0;
	while(true) {
		if(offset >= int(gas_trace.size())) {
			return ERROR; // something must be wrong
		}
		int64_t g = gas_trace[offset];
		offset++;
		bool is_call = (g&MSB64) != 0;
		g = g & ~MSB64;
		if(is_call) { // g is gas_given to sub
			gas_consumed.push_back(CALL_EVM);
			int64_t consumed = fill_gas_consumed(gas_consumed, gas_trace, offset, g);
			if(consumed == ERROR) {
				return ERROR; // something must be wrong
			}
			gas_used_by_sub += consumed;
			//std::cerr<<" gas_used_by_sub "<<gas_used_by_sub<<std::endl;
		} else { // g is gas_left
			int64_t gas_used_by_me = gas_given - gas_used_by_sub - g;
			//std::cerr<<std::dec<<" gas_given "<<gas_given<<" g "<<g<<" gas_used_by_me "<<gas_used_by_me<<std::endl;
			gas_consumed.push_back(gas_used_by_me);
			int64_t gas_used = gas_given - g;
			return gas_used;
		}
	}
	return ERROR; // must not reach here
}

// Given initial_gas and a gas consumption history recorded in gas_consumed[offset:], returns how much gas will be left
// after applying the consumption history, returning negative value means initial_gas is not enough.
int64_t get_gas_left(std::vector<int64_t>& gas_consumed, int& offset, int64_t initial_gas, int& curr_depth, int& max_depth) {
	int64_t gas_left = initial_gas;
	while(true) {
		if(offset >= int(gas_consumed.size())) {
			return ERROR; // something must be wrong
		}
		int64_t g = gas_consumed[offset];
		offset++;
		if(g == CALL_EVM) {
			int64_t gas_reserved = gas_left/64; //gas reserved for me, not given to sub
			curr_depth++;
			if(max_depth < curr_depth) max_depth = curr_depth;
			gas_left = get_gas_left(gas_consumed, offset, gas_left-gas_reserved, curr_depth, max_depth);
			curr_depth--;
			if(gas_left < 0) {
				break;
			}
			gas_left += gas_reserved;
		} else {
			gas_left -= g; // g is gas consumed by me
			break;
		}
	}
	return gas_left;
}

// Given a TX's gas_trace with gas_given&gas_left information and init_guess, estimate a low enough gas limit which allows
// this TX to finish without out-of-gas.
int64_t estimate_gas_with_trace(std::vector<int64_t>& gas_trace, int64_t init_guess, int& call_depth) {
	std::vector<int64_t> gas_consumed;
	gas_consumed.reserve(gas_trace.size());
	if(gas_trace.size() < 2 || (gas_trace[0] & MSB64) == 0 ) {
		return ERROR; // something must be wrong
	}
	//for(int i=0; i<gas_trace.size(); i++) {
	//	std::cout<<"Hehe"<<i<<" "<<(gas_trace[i]&~MSB64)<<std::endl;
	//}
	int offset = 1;
	gas_consumed.push_back(-1);
	int64_t ret = fill_gas_consumed(gas_consumed, gas_trace, offset, gas_trace[0]&~MSB64);
	if(ret == ERROR) {
		return ERROR; // something must be wrong
	}
	if(offset != int(gas_consumed.size())) {
		return ERROR; // something must be wrong
	}
	//for(int i=0; i<gas_consumed.size(); i++) {
	//	std::cout<<"Haha"<<i<<" "<<(gas_trace[i]&~MSB64)<<" "<<gas_consumed[i]<<std::endl;
	//}
	int64_t high=init_guess, mid=init_guess/2, low=0;
	// now we use binary search to get a low enough guess
	int curr_depth = 1, max_depth = 1;
	while(high - low > THRES) {
		//std::cout<<"hi mid lo "<<high<<" "<<mid<<" "<<low<<std::endl;
		int offset = 1;
		int64_t gas_left = get_gas_left(gas_consumed, offset, mid, curr_depth, max_depth);
		//std::cout<<"gas_left "<<gas_left<<std::endl;
		if(gas_left < 0) {
			low = mid;
		} else if(gas_left < THRES) {
			return mid;
		} else {
			high = mid;
		}
		mid = (low+high)/2;
	}
	call_depth = max_depth;
	if(high > mid + THRES) {
		return high;
	}
	return mid + THRES;
}

int64_t tx_control::estimate_gas(int64_t init_guess) {
	int call_depth=0;
	int64_t gas_estimated = estimate_gas_with_trace(this->gas_trace, init_guess, call_depth);
	//std::cout<<"WE_ESTIMATE CALL DEPTH "<<call_depth<<std::endl;
	//gawk 'BEGIN {RS="DEPTH";} $5>6000&&$5/$3>0.05&&$1<50 {print $3" "$5" "($5/$3)" "$6" "$1"\n---------"}' estimate.log 
	return gas_estimated;
}

