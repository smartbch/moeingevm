// evmone: Fast Ethereum Virtual Machine implementation
// Copyright 2019-2020 The evmone Authors.
// SPDX-License-Identifier: Apache-2.0

#include "execution.hpp"
#include "analysis.hpp"
#include <memory>
#include <string>
#include <iostream>
#include <iomanip>
#include <cstdlib>

namespace evmone
{
std::string opName(int op) {
	auto names = evmc_get_instruction_names_table(EVMC_LONDON);
	auto name = names[op%256];
	if(name) {
		return std::string(name);
	}
	return std::string("UNKNOWN");
}

evmc_result execute(AdvancedExecutionState& state, const AdvancedCodeAnalysis& analysis) noexcept
{
    static bool checked_env = false;
    static bool withgas, nostack, notrace, noinstlog;
    if(!checked_env) {
        checked_env = true;
        withgas = std::getenv("WITHGAS") != nullptr;
        nostack = std::getenv("NOSTACK") != nullptr;
        notrace = std::getenv("NOTRACE") != nullptr;
        noinstlog = std::getenv("NOINSTLOG") != nullptr;
    }

    state.analysis = &analysis;  // Allow accessing the analysis by instructions.

    const auto* instr = &state.analysis->instrs[0];  // Start with the first instruction.
    while (instr != nullptr) {
        auto pc = instr->pc;
        auto op = instr->op;
        auto name = opName(op);
        
	if(!notrace) {
            if(op!=OPX_BEGINBLOCK && !noinstlog) {
                std::cerr<<"====*===="<<std::endl;
                std::cerr<<"PC:"<<std::dec<<pc<<" OP: "<<name<<" "<<std::dec<<int(op);
                if(withgas) {
                    std::cerr<<" gas 0x"<<std::hex<<state.gas_left;
                }
                std::cerr<<std::endl;
            }
        }
        
        instr = instr->fn(instr, state);
        
        if(op!=OPX_BEGINBLOCK && !nostack) {
            for(int i = state.stack.size() - 1; i >= 0; i--) {
		std::cerr<<"0x"<<intx::hex(state.stack[i])<<std::endl;
            }
        }
    }


    const auto gas_left =
        (state.status == EVMC_SUCCESS || state.status == EVMC_REVERT) ? state.gas_left : 0;

    return evmc::make_result(
        state.status, gas_left, state.memory.data() + state.output_offset, state.output_size);
}

evmc_result execute(evmc_vm* /*unused*/, const evmc_host_interface* host, evmc_host_context* ctx,
    evmc_revision rev, const evmc_message* msg, const uint8_t* code, size_t code_size) noexcept
{
    bool nodiasm = std::getenv("NODIASM") != nullptr;
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

    std::cerr<<"Caller: 0x"<<intx::hex(intx::be::load<uint256>(msg->sender))<<std::endl;
    std::cerr<<"Callee: 0x"<<intx::hex(intx::be::load<uint256>(msg->destination))<<std::endl;
	
    const auto analysis = analyze(rev, code, code_size);
    auto state = std::make_unique<AdvancedExecutionState>(*msg, rev, *host, ctx, code, code_size);
    return execute(*state, analysis);
}
}  // namespace evmone
