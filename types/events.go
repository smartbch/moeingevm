package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	modbtypes "github.com/smartbch/moeingdb/types"
)

/*
type ChainEvent struct {
	Block *types.Block
	Hash  common.Hash
	Logs  []*types.Log
}
*/
type ChainEvent struct {
	BlockHeader *Header
	Block       *modbtypes.Block
	Hash        gethcmn.Hash
	Logs        []*gethtypes.Log
	// TODO: define more fields
}

func BlockToChainEvent(mdbBlock *modbtypes.Block) ChainEvent {
	return ChainEvent{
		Hash: mdbBlock.BlockHash,
		BlockHeader: &Header{
			Number:    uint64(mdbBlock.Height),
			BlockHash: mdbBlock.BlockHash,
		},
		Block: mdbBlock,
		Logs:  collectAllGethLogs(mdbBlock),
	}
}

func collectAllGethLogs(mdbBlock *modbtypes.Block) []*gethtypes.Log {
	logs := make([]*gethtypes.Log, 0, 8)
	for _, mdbTx := range mdbBlock.TxList {
		for _, mdbLog := range mdbTx.LogList {
			logs = append(logs, &gethtypes.Log{
				Address: mdbLog.Address,
				Topics:  ToGethHashes(mdbLog.Topics),
			})
		}
	}
	return logs
}
