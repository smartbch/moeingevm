package types

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	coretypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/smartbch/moeingevm/utils"
)

//	uint64_t get_creation_counter(uint8_t n) {
//	21, 1-byte
//	account_info get_account(const evmc_address& addr) {
//	23, 20-byte
//	bytes get_bytecode(const evmc_address& addr, evmc_bytes32* codehash) {
//	25, 20-byte
//	bytes get_value(uint64_t seq, const evmc_bytes32& key) {
//	27, 8-byte, 32-byte

const CREATION_COUNTER_KEY byte = 21
const ACCOUNT_KEY byte = 23
const BYTECODE_KEY byte = 25
const VALUE_KEY byte = 27
const CURR_BLOCK_KEY byte = 29

var StandbyTxQueueKey [8]byte = [8]byte{255, 255, 255, 255, 255, 255, 255, 0}

const TOO_OLD_THRESHOLD uint64 = 10

const IGNORE_TOO_OLD_TX int = 1024
const FAILED_TO_COMMIT int = 1025
const ACCOUNT_NOT_EXIST int = 1026
const TX_NONCE_TOO_SMALL int = 1027
const TX_NONCE_TOO_LARGE int = 1029

func GetCreationCounterKey(lsb uint8) []byte {
	bz := make([]byte, 2)
	bz[0] = CREATION_COUNTER_KEY
	bz[1] = lsb
	return bz
}

func GetAccountKey(addr common.Address) []byte {
	bz := make([]byte, 1, 1+len(addr))
	bz[0] = ACCOUNT_KEY
	return append(bz, addr[:]...)
}

func GetBytecodeKey(addr common.Address) []byte {
	bz := make([]byte, 1, 1+len(addr))
	bz[0] = BYTECODE_KEY
	return append(bz, addr[:]...)
}

func GetValueKey(seq uint64, key string) []byte {
	if len(key) != 32 {
		panic("Invalid length for key")
	}
	bz := make([]byte, 9, 9+32)
	bz[0] = VALUE_KEY
	binary.BigEndian.PutUint64(bz[1:], seq)
	return append(bz, []byte(key)...)
}

func GetStandbyTxKey(num uint64) []byte {
	var buf [8]byte
	num += uint64(128+64) << 56 // raise it to the non-rabbit range
	binary.BigEndian.PutUint64(buf[:], num)
	return buf[:]
}

type EvmLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

type BlockInfo struct {
	Coinbase   [20]byte
	Hash       [32]byte
	Number     int64
	Timestamp  int64
	GasLimit   int64
	Difficulty [32]byte
	ChainId    [32]byte
}

type BasicTx struct {
	From     common.Address
	To       common.Address
	Value    [32]byte
	GasPrice [32]byte
	Gas      uint64
	Data     []byte
	Nonce    uint64
}

type TxToRun struct {
	BasicTx
	HashID common.Hash
	Height uint64
}

func (tx TxToRun) ToBytes() []byte {
	res := make([]byte, 0, 32+20+20+8+32+32+8+len(tx.Data)+8)
	res = append(res, tx.HashID[:]...)
	res = append(res, tx.From[:]...)
	res = append(res, tx.To[:]...)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], tx.Height)
	res = append(res, buf[:]...)
	res = append(res, tx.Value[:]...)
	res = append(res, tx.GasPrice[:]...)
	binary.BigEndian.PutUint64(buf[:], tx.Gas)
	res = append(res, buf[:]...)
	res = append(res, tx.Data...)
	var nonceBuf [8]byte
	binary.BigEndian.PutUint64(nonceBuf[:], tx.Nonce)
	res = append(res, nonceBuf[:]...)
	return res
}

func (tx *TxToRun) FromBytes(bz []byte) {
	copy(tx.HashID[:], bz)
	bz = bz[32:]
	copy(tx.From[:], bz)
	bz = bz[20:]
	copy(tx.To[:], bz)
	bz = bz[20:]
	tx.Height = binary.BigEndian.Uint64(bz[:8])
	bz = bz[8:]
	copy(tx.Value[:], bz)
	bz = bz[32:]
	copy(tx.GasPrice[:], bz)
	bz = bz[32:]
	tx.Gas = binary.BigEndian.Uint64(bz[:8])
	bz = bz[8:]
	tx.Data = append([]byte{}, bz[:len(bz)-8]...)
	bz = bz[len(bz)-8:]
	tx.Nonce = binary.BigEndian.Uint64(bz[:])
}

func (tx *TxToRun) FromGethTx(gethTx *coretypes.Transaction, sender common.Address, height uint64) {
	tx.HashID = gethTx.Hash()
	tx.From = sender
	if to := gethTx.To(); to != nil {
		tx.To = *to
	}
	tx.Height = height
	tx.Gas = gethTx.Gas()
	tx.Data = gethTx.Data()
	tx.Nonce = gethTx.Nonce()
	copy(tx.Value[:], utils.BigIntToSlice32(gethTx.Value()))
	copy(tx.GasPrice[:], utils.BigIntToSlice32(gethTx.GasPrice()))
}
