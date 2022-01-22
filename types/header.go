package types

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethcore "github.com/ethereum/go-ethereum/core/types"
)

type Header struct {
	Number     hexutil.Uint64  `json:"number"`
	BlockHash  gethcmn.Hash    `json:"hash"`
	ParentHash gethcmn.Hash    `json:"parentHash"`
	Bloom      gethcore.Bloom  `json:"logsBloom"`
	TxRoot     gethcmn.Hash    `json:"transactionsRoot"`
	StateRoot  gethcmn.Hash    `json:"stateRoot"`
	Miner      gethcmn.Address `json:"miner"`
	GasUsed    hexutil.Uint64  `json:"gasUsed"`
	Timestamp  hexutil.Uint64  `json:"timestamp"`
	//UncleHash   gethcmn.Hash   `json:"sha3Uncles"`
	//ReceiptHash gethcmn.Hash   `json:"receiptsRoot"`
	//Difficulty  *hexutil.Big   `json:"difficulty"`
	//GasLimit    hexutil.Uint64 `json:"gasLimit"`
}

func (h *Header) Hash() gethcmn.Hash {
	return h.BlockHash
}
