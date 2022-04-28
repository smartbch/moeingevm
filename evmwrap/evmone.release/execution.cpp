// evmone: Fast Ethereum Virtual Machine implementation
// Copyright 2019-2020 The evmone Authors.
// SPDX-License-Identifier: Apache-2.0

#include "execution.hpp"
#include "analysis.hpp"
#include "cache.hpp"
#include "../host_bridge/host_context.h"
#include <memory>
#include <cstdlib>

namespace evmone
{
evmc_result execute(AdvancedExecutionState& state, const AdvancedCodeAnalysis& analysis) noexcept
{
    state.analysis = &analysis;  // Allow accessing the analysis by instructions.

    const auto* instr = &state.analysis->instrs[0];  // Start with the first instruction.
    while (instr != nullptr)
        instr = instr->fn(instr, state);

    const auto gas_left =
        (state.status == EVMC_SUCCESS || state.status == EVMC_REVERT) ? state.gas_left : 0;

    return evmc::make_result(
        state.status, gas_left, state.memory.data() + state.output_offset, state.output_size);
}

typedef Cache<AdvancedCodeAnalysis> AnalysisCache;
AnalysisCache CacheShards[AnalysisCache::SHARD_COUNT];

evmc_result execute(evmc_vm* /*unused*/, const evmc_host_interface* host, evmc_host_context* ctx,
    evmc_revision rev, const evmc_message* msg, const uint8_t* code, size_t code_size) noexcept
{
    static int disable_cache = -1;
    if(disable_cache<0) {
        disable_cache = (std::getenv("EVMONE_DISABLE_ANALYSIS_CACHE") == nullptr)? 0 : 1;
    }
    auto state = std::make_unique<AdvancedExecutionState>(*msg, rev, *host, ctx, code, code_size);
    if(disable_cache>0) {
        auto analysis = analyze(rev, code, code_size);
        return execute(*state, analysis);
    }
    std::string key((const char*)(ctx->get_codehash().bytes), 32);
    int sid = int(uint8_t(key[31])) % AnalysisCache::SHARD_COUNT; //shard id
    const auto height = static_cast<uint32_t>(state->host.get_tx_context().block_number);
    const AdvancedCodeAnalysis& analysis = CacheShards[sid].borrow(key);
    evmc_result res;
    if(analysis.instrs.size() > 0) { // cache hit
        res = execute(*state, analysis);
        CacheShards[sid].give_back(key, height);
    } else  { // cache miss
        auto new_analysis = analyze(rev, code, code_size);
        res = execute(*state, new_analysis);
        if(msg->kind != EVMC_CREATE && msg->kind != EVMC_CREATE2) { // add to cache
            CacheShards[sid].add(key, new_analysis, height);
        }
    }
    return res;
}
}  // namespace evmone
