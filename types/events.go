package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
		Hash:        mdbBlock.BlockHash,
		BlockHeader: toBlockHeader(mdbBlock),
		Block:       mdbBlock,
		Logs:        collectAllGethLogs(mdbBlock),
	}
}

func collectAllGethLogs(mdbBlock *modbtypes.Block) []*gethtypes.Log {
	logs := make([]*gethtypes.Log, 0, 8)
	for _, mdbTx := range mdbBlock.TxList {
		tx := &Transaction{}
		_, err := tx.UnmarshalMsg(mdbTx.Content)
		if err != nil { // ignore error
			//panic(err)
			println("failed to unmarshal tx:", err.Error())
		}

		for i, mdbLog := range mdbTx.LogList {
			gethLog := &gethtypes.Log{
				Address:     mdbLog.Address,
				Topics:      ToGethHashes(mdbLog.Topics),
				BlockNumber: uint64(mdbBlock.Height),
				BlockHash:   tx.BlockHash,
				TxHash:      tx.Hash,
				TxIndex:     uint(tx.TransactionIndex),
				Index:       uint(i),
			}
			if i < len(tx.Logs) {
				gethLog.Data = tx.Logs[i].Data
			}
			logs = append(logs, gethLog)
		}
	}
	return logs
}

func toBlockHeader(mdbBlock *modbtypes.Block) *Header {
	block := &Block{}
	_, err := block.UnmarshalMsg(mdbBlock.BlockInfo)
	if err != nil { // ignore error
		//panic(err)
		println("failed to unmarshal block:", err.Error())
	}

	return &Header{
		Number:     hexutil.Uint64(mdbBlock.Height),
		BlockHash:  mdbBlock.BlockHash,
		ParentHash: block.ParentHash,
		Bloom:      block.LogsBloom,
		TxRoot:     block.TransactionsRoot,
		StateRoot:  block.StateRoot,
		Miner:      block.Miner,
		GasUsed:    hexutil.Uint64(block.GasUsed),
		Timestamp:  hexutil.Uint64(block.Timestamp),
	}
}
