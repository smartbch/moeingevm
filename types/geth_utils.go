package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

func ToGethLogs(logs []Log) []*gethtypes.Log {
	gethLogs := make([]*gethtypes.Log, len(logs))
	for i, log := range logs {
		gethLogs[i] = ToGethLog(log)
	}
	return gethLogs
}

func ToGethLog(log Log) *gethtypes.Log {
	return &gethtypes.Log{
		Address:     log.Address,
		Topics:      ToGethHashes(log.Topics),
		Data:        log.Data,
		BlockNumber: log.BlockNumber,
		BlockHash:   log.BlockHash,
		TxHash:      log.TxHash,
		TxIndex:     log.TxIndex,
		Index:       log.Index,
		Removed:     log.Removed,
	}
}

func ToGethHashes(rawHashes [][32]byte) []gethcmn.Hash {
	gethHashes := make([]gethcmn.Hash, len(rawHashes))
	for i, topic := range rawHashes {
		gethHashes[i] = topic
	}
	return gethHashes
}

func FromGethHashes(gethHashes []gethcmn.Hash) [][32]byte {
	rawHashes := make([][32]byte, len(gethHashes))
	for i, gethTopic := range gethHashes {
		rawHashes[i] = gethTopic
	}
	return rawHashes
}

func FromGethAddreses(gethAddresses []gethcmn.Address) [][20]byte {
	rawAddresses := make([][20]byte, len(gethAddresses))
	for i, gethAddr := range gethAddresses {
		rawAddresses[i] = gethAddr
	}
	return rawAddresses
}
