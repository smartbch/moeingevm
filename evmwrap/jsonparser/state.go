package main

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

/*
type GenesisAlloc map[common.Address]GenesisAccount

type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}
*/

type BlockTest struct {
	Json btJSON
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *BlockTest) UnmarshalJSON(in []byte) error {
	return json.Unmarshal(in, &t.Json)
}

type btJSON struct {
	Blocks     []btBlock             `json:"blocks"`
	Genesis    btHeader              `json:"genesisBlockHeader"`
	Pre        core.GenesisAlloc     `json:"pre"`
	Post       core.GenesisAlloc     `json:"postState"`
	BestBlock  common.UnprefixedHash `json:"lastblockhash"`
	Network    string                `json:"network"`
	SealEngine string                `json:"sealEngine"`
}

//go:generate gencodec -type stTransaction -field-override stTransactionMarshaling -out gen_sttransaction.go

//nolint:unused
type stTransaction struct {
	Nonce    uint64   `json:"nonce"`
	To       string   `json:"to"`
	Data     string   `json:"data"`
	Value    *big.Int `json:"value"`
	GasLimit uint64   `json:"gasLimit"`
	GasPrice *big.Int `json:"gasPrice"`
	V        *big.Int `json:"v"`
	R        *big.Int `json:"r"`
	S        *big.Int `json:"s"`
}

type stTransactionMarshaling struct {
	GasPrice *math.HexOrDecimal256
	V        *math.HexOrDecimal256
	R        *math.HexOrDecimal256
	S        *math.HexOrDecimal256
	Value    *math.HexOrDecimal256
	Nonce    math.HexOrDecimal64
	GasLimit math.HexOrDecimal64
}

type btBlock struct {
	Rlp string `json:"rlp"`
	//Tx stTransaction `json:"transactions"`
	Header btHeader `json:"blockHeader"`
}

//go:generate gencodec -type btHeader -field-override btHeaderMarshaling -out gen_btheader.go
type btHeader struct {
	Bloom            types.Bloom
	Coinbase         common.Address `json:"coinbase"`
	MixHash          common.Hash
	Nonce            types.BlockNonce
	Number           *big.Int `json:"number"`
	Hash             common.Hash
	ParentHash       common.Hash
	ReceiptTrie      common.Hash
	StateRoot        common.Hash
	TransactionsTrie common.Hash
	UncleHash        common.Hash
	ExtraData        []byte
	Difficulty       *big.Int
	GasLimit         uint64
	GasUsed          uint64
	Timestamp        uint64 `json:"timestamp"`
}

type btHeaderMarshaling struct {
	ExtraData  hexutil.Bytes
	Number     *math.HexOrDecimal256
	Difficulty *math.HexOrDecimal256
	GasLimit   math.HexOrDecimal64
	GasUsed    math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
}

func (bb *btBlock) decode() (*types.Block, error) {
	data, err := hexutil.Decode(bb.Rlp)
	if err != nil {
		return nil, err
	}
	var b types.Block
	err = rlp.DecodeBytes(data, &b)
	return &b, err
}
