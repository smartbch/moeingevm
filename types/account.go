package types

import (
	"encoding/binary"
	"github.com/holiman/uint256"

	"github.com/moeing-chain/MoeingEVM/utils"
)
type AccountInfo struct {
	data []byte
}

func NewAccountInfo(data []byte) *AccountInfo {
	if len(data) != 48 {
		panic("Invalid length for AccountInfo")
	}
	return &AccountInfo{data: data}
}

func ZeroAccountInfo() *AccountInfo {
	return &AccountInfo{data: make([]byte, 48)}
}

func (info *AccountInfo) BalanceSlice() []byte {
	return info.data[0:32]
}

func (info *AccountInfo) NonceSlice() []byte {
	return info.data[32:40]
}

func (info *AccountInfo) SequenceSlice() []byte {
	return info.data[40:48]
}

func (info *AccountInfo) Bytes() []byte {
	return info.data
}

func (info *AccountInfo) Balance() *uint256.Int {
	return utils.U256FromSlice32(info.BalanceSlice())
}

func (info *AccountInfo) UpdateBalance(newBalance *uint256.Int) {
	copy(info.BalanceSlice(), utils.U256ToSlice32(newBalance))
}

func (info *AccountInfo) Nonce() uint64 {
	return binary.BigEndian.Uint64(info.NonceSlice())
}

func (info *AccountInfo) UpdateNonce(newNonce uint64) {
	binary.BigEndian.PutUint64(info.NonceSlice(), newNonce)
}

func (info *AccountInfo) Sequence() uint64 {
	return binary.BigEndian.Uint64(info.SequenceSlice())
}

func (info *AccountInfo) UpdateSequence(newSeq uint64) {
	binary.BigEndian.PutUint64(info.SequenceSlice(), newSeq)
}
