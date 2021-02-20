// evmone: Fast Ethereum Virtual Machine implementation
// Copyright 2019 Pawel Bylica.
// Licensed under the Apache License, Version 2.0.

#include "execution.hpp"
#include "analysis.hpp"
#include <memory>
#include <string>
#include <iostream>
#include <iomanip>
#include <cstdlib>


namespace evmone
{
std::string opName(int op);

void print_uint256(uint256 elem) {
	std::cerr<<"0x";
	if(elem.hi.hi!=0)std::cerr<< std::hex << elem.hi.hi;
	if(elem.hi.hi==0) {
		if(elem.hi.lo!=0) std::cerr<< std::hex <<elem.hi.lo;
	} else {
		std::cerr<< std::hex << std::setw(16) << std::setfill('0') <<elem.hi.lo;
	}
	if(elem.hi.hi==0 && elem.hi.lo==0) {
		if(elem.lo.hi!=0) std::cerr<< std::hex <<elem.lo.hi;
	} else {
		std::cerr<< std::hex << std::setw(16) << std::setfill('0') <<elem.lo.hi;
	}
	if(elem.hi.hi==0 && elem.hi.lo==0 && elem.lo.hi==0) {
	    std::cerr<< std::hex <<elem.lo.lo<<std::endl;
	} else {
	    std::cerr<< std::hex << std::setw(16) << std::setfill('0') <<elem.lo.lo<<std::endl;
	}
}
	
evmc_result execute(evmc_vm* /*unused*/, const evmc_host_interface* host, evmc_host_context* ctx,
    evmc_revision rev, const evmc_message* msg, const uint8_t* code, size_t code_size) noexcept
{
    static bool checked_env = false;
    static bool withgas, nodiasm, nostack, notrace, noinstlog;
    if(!checked_env) {
        checked_env = true;
        withgas = std::getenv("WITHGAS") != nullptr;
        nodiasm = std::getenv("NODIASM") != nullptr;
        nostack = std::getenv("NOSTACK") != nullptr;
        notrace = std::getenv("NOTRACE") != nullptr;
        noinstlog = std::getenv("NOINSTLOG") != nullptr;
    }
    if(!nodiasm) {
        std::cerr<<"-=-=-=- Now execute following code ("<<size_t(code)<<" "<<code_size<<"):"<<std::endl;
        for (size_t pc=0; pc<code_size; pc++) {
            auto op = code[pc];
            auto name = opName(op);
            std::cerr<<"PC:"<<std::dec<<pc<<" OP: "<<name<<" "<<int(op)<<std::endl;
            if (OP_PUSH1 <= op && op <= OP_PUSH32) {
                auto end = pc + (op-OP_PUSH1) + 1;
                for (pc++; pc < end; pc++) {
                    std::cerr<<"   "<<pc<<" : "<<std::hex<<int(code[pc])<<std::endl;
                }
                std::cerr<<"   "<<pc<<" : "<<std::hex<<int(code[pc])<<std::endl;
            }
        }
    }

    std::cerr<<"Caller: ";
    print_uint256(intx::be::load<uint256>(msg->sender));
    std::cerr<<"Callee: ";
    print_uint256(intx::be::load<uint256>(msg->destination));
	
    auto analysis = analyze(rev, code, code_size);

    auto state = std::make_unique<execution_state>();
    state->analysis = &analysis;
    state->msg = msg;
    state->code = code;
    state->code_size = code_size;
    state->host = evmc::HostContext{*host, ctx};
    state->gas_left = msg->gas;
    state->rev = rev;

    const auto* instr = &state->analysis->instrs[0];
    //while (instr != nullptr)
    //    instr = instr->fn(instr, *state);
    while (instr != nullptr) {
        auto pc = instr->pc;
        auto op = instr->op;
        auto name = opName(op);
        
	if(!notrace) {
            if(op!=OPX_BEGINBLOCK && !noinstlog) {
                std::cerr<<"====*===="<<std::endl;
                std::cerr<<"PC:"<<std::dec<<pc<<" OP: "<<name<<" "<<std::dec<<int(op);
                if(withgas) {
                    std::cerr<<" gas 0x"<<std::hex<<state->gas_left;
                }
                std::cerr<<std::endl;
            }
        }
        
        instr = instr->fn(instr, *state);
        
        if(op!=OPX_BEGINBLOCK && !nostack) {
            for(int i = state->stack.size() - 1; i >= 0; i--) {
                auto elem = state->stack[i];
                print_uint256(elem);
            }
        }
    }

    const auto gas_left =
        (state->status == EVMC_SUCCESS || state->status == EVMC_REVERT) ? state->gas_left : 0;

    return evmc::make_result(
        state->status, gas_left, &state->memory[state->output_offset], state->output_size);
}
std::string opName(int op) {
    switch(op) {
    case OP_STOP: return std::string("STOP");
    case OP_ADD: return std::string("ADD");
    case OP_MUL: return std::string("MUL");
    case OP_SUB: return std::string("SUB");
    case OP_DIV: return std::string("DIV");
    case OP_SDIV: return std::string("SDIV");
    case OP_MOD: return std::string("MOD");
    case OP_SMOD: return std::string("SMOD");
    case OP_ADDMOD: return std::string("ADDMOD");
    case OP_MULMOD: return std::string("MULMOD");
    case OP_EXP: return std::string("EXP");
    case OP_SIGNEXTEND: return std::string("SIGNEXTEND");
    case OP_LT: return std::string("LT");
    case OP_GT: return std::string("GT");
    case OP_SLT: return std::string("SLT");
    case OP_SGT: return std::string("SGT");
    case OP_EQ: return std::string("EQ");
    case OP_ISZERO: return std::string("ISZERO");
    case OP_AND: return std::string("AND");
    case OP_OR: return std::string("OR");
    case OP_XOR: return std::string("XOR");
    case OP_NOT: return std::string("NOT");
    case OP_BYTE: return std::string("BYTE");
    case OP_SHL: return std::string("SHL");
    case OP_SHR: return std::string("SHR");
    case OP_SAR: return std::string("SAR");
    case OP_SHA3: return std::string("SHA3");
    case OP_ADDRESS: return std::string("ADDRESS");
    case OP_BALANCE: return std::string("BALANCE");
    case OP_ORIGIN: return std::string("ORIGIN");
    case OP_CALLER: return std::string("CALLER");
    case OP_CALLVALUE: return std::string("CALLVALUE");
    case OP_CALLDATALOAD: return std::string("CALLDATALOAD");
    case OP_CALLDATASIZE: return std::string("CALLDATASIZE");
    case OP_CALLDATACOPY: return std::string("CALLDATACOPY");
    case OP_CODESIZE: return std::string("CODESIZE");
    case OP_CODECOPY: return std::string("CODECOPY");
    case OP_GASPRICE: return std::string("GASPRICE");
    case OP_EXTCODESIZE: return std::string("EXTCODESIZE");
    case OP_EXTCODECOPY: return std::string("EXTCODECOPY");
    case OP_RETURNDATASIZE: return std::string("RETURNDATASIZE");
    case OP_RETURNDATACOPY: return std::string("RETURNDATACOPY");
    case OP_EXTCODEHASH: return std::string("EXTCODEHASH");
    case OP_BLOCKHASH: return std::string("BLOCKHASH");
    case OP_COINBASE: return std::string("COINBASE");
    case OP_TIMESTAMP: return std::string("TIMESTAMP");
    case OP_NUMBER: return std::string("NUMBER");
    case OP_DIFFICULTY: return std::string("DIFFICULTY");
    case OP_GASLIMIT: return std::string("GASLIMIT");
    case OP_CHAINID: return std::string("CHAINID");
    case OP_SELFBALANCE: return std::string("SELFBALANCE");
    case OP_POP: return std::string("POP");
    case OP_MLOAD: return std::string("MLOAD");
    case OP_MSTORE: return std::string("MSTORE");
    case OP_MSTORE8: return std::string("MSTORE8");
    case OP_SLOAD: return std::string("SLOAD");
    case OP_SSTORE: return std::string("SSTORE");
    case OP_JUMP: return std::string("JUMP");
    case OP_JUMPI: return std::string("JUMPI");
    case OP_PC: return std::string("PC");
    case OP_MSIZE: return std::string("MSIZE");
    case OP_GAS: return std::string("GAS");
    case OP_JUMPDEST: return std::string("JUMPDEST");
    case OP_PUSH1: return std::string("PUSH1");
    case OP_PUSH2: return std::string("PUSH2");
    case OP_PUSH3: return std::string("PUSH3");
    case OP_PUSH4: return std::string("PUSH4");
    case OP_PUSH5: return std::string("PUSH5");
    case OP_PUSH6: return std::string("PUSH6");
    case OP_PUSH7: return std::string("PUSH7");
    case OP_PUSH8: return std::string("PUSH8");
    case OP_PUSH9: return std::string("PUSH9");
    case OP_PUSH10: return std::string("PUSH10");
    case OP_PUSH11: return std::string("PUSH11");
    case OP_PUSH12: return std::string("PUSH12");
    case OP_PUSH13: return std::string("PUSH13");
    case OP_PUSH14: return std::string("PUSH14");
    case OP_PUSH15: return std::string("PUSH15");
    case OP_PUSH16: return std::string("PUSH16");
    case OP_PUSH17: return std::string("PUSH17");
    case OP_PUSH18: return std::string("PUSH18");
    case OP_PUSH19: return std::string("PUSH19");
    case OP_PUSH20: return std::string("PUSH20");
    case OP_PUSH21: return std::string("PUSH21");
    case OP_PUSH22: return std::string("PUSH22");
    case OP_PUSH23: return std::string("PUSH23");
    case OP_PUSH24: return std::string("PUSH24");
    case OP_PUSH25: return std::string("PUSH25");
    case OP_PUSH26: return std::string("PUSH26");
    case OP_PUSH27: return std::string("PUSH27");
    case OP_PUSH28: return std::string("PUSH28");
    case OP_PUSH29: return std::string("PUSH29");
    case OP_PUSH30: return std::string("PUSH30");
    case OP_PUSH31: return std::string("PUSH31");
    case OP_PUSH32: return std::string("PUSH32");
    case OP_DUP1: return std::string("DUP1");
    case OP_DUP2: return std::string("DUP2");
    case OP_DUP3: return std::string("DUP3");
    case OP_DUP4: return std::string("DUP4");
    case OP_DUP5: return std::string("DUP5");
    case OP_DUP6: return std::string("DUP6");
    case OP_DUP7: return std::string("DUP7");
    case OP_DUP8: return std::string("DUP8");
    case OP_DUP9: return std::string("DUP9");
    case OP_DUP10: return std::string("DUP10");
    case OP_DUP11: return std::string("DUP11");
    case OP_DUP12: return std::string("DUP12");
    case OP_DUP13: return std::string("DUP13");
    case OP_DUP14: return std::string("DUP14");
    case OP_DUP15: return std::string("DUP15");
    case OP_DUP16: return std::string("DUP16");
    case OP_SWAP1: return std::string("SWAP1");
    case OP_SWAP2: return std::string("SWAP2");
    case OP_SWAP3: return std::string("SWAP3");
    case OP_SWAP4: return std::string("SWAP4");
    case OP_SWAP5: return std::string("SWAP5");
    case OP_SWAP6: return std::string("SWAP6");
    case OP_SWAP7: return std::string("SWAP7");
    case OP_SWAP8: return std::string("SWAP8");
    case OP_SWAP9: return std::string("SWAP9");
    case OP_SWAP10: return std::string("SWAP10");
    case OP_SWAP11: return std::string("SWAP11");
    case OP_SWAP12: return std::string("SWAP12");
    case OP_SWAP13: return std::string("SWAP13");
    case OP_SWAP14: return std::string("SWAP14");
    case OP_SWAP15: return std::string("SWAP15");
    case OP_SWAP16: return std::string("SWAP16");
    case OP_LOG0: return std::string("LOG0");
    case OP_LOG1: return std::string("LOG1");
    case OP_LOG2: return std::string("LOG2");
    case OP_LOG3: return std::string("LOG3");
    case OP_LOG4: return std::string("LOG4");
    case OP_CREATE: return std::string("CREATE");
    case OP_CALL: return std::string("CALL");
    case OP_CALLCODE: return std::string("CALLCODE");
    case OP_RETURN: return std::string("RETURN");
    case OP_DELEGATECALL: return std::string("DELEGATECALL");
    case OP_CREATE2: return std::string("CREATE2");
    case OP_STATICCALL: return std::string("STATICCALL");
    case OP_REVERT: return std::string("REVERT");
    case OP_INVALID: return std::string("INVALID");
    case OP_SELFDESTRUCT: return std::string("SELFDESTRUCT");
    default: return std::string("UNKNOWN");
    }
}


}  // namespace evmone
