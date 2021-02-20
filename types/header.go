package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	gethcore "github.com/ethereum/go-ethereum/core/types"
)

type Header struct {
	Number    uint64
	BlockHash gethcmn.Hash
	Bloom     gethcore.Bloom
	// TODO: add more fields
}

func (h *Header) Hash() gethcmn.Hash {
	return h.BlockHash
}
