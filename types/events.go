package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	modbtypes "github.com/moeing-chain/MoeingDB/types"
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
