package types

import (
	"bytes"
	"errors"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/smartbch/moeingads/store/rabbit"
	modbtypes "github.com/smartbch/moeingdb/types"
)

var (
	ErrAccountNotExist        = errors.New("account does not exist")
	ErrNonceTooSmall          = errors.New("tx nonce is smaller than the account nonce")
	ErrSameNonceAlredyInBlock = errors.New("tx with same nonce already in block")
	ErrNonceTooLarge          = errors.New("tx nonce is larger than the account nonce")
	ErrTooManyEntries         = errors.New("too many candidicate entries to be returned, please limit the difference between startHeight and endHeight")
)

type Context struct {
	Rbt              *rabbit.RabbitStore
	Db               modbtypes.DB
	Height           int64
	XHedgeForkBlock  int64
	ShaGateForkBlock int64
}

func NewContext(rbt *rabbit.RabbitStore, db modbtypes.DB) *Context {
	return &Context{
		Rbt:              rbt,
		Db:               db,
		XHedgeForkBlock:  math.MaxInt64,
		ShaGateForkBlock: math.MaxInt64,
	}
}

func (c *Context) WithRbt(rabbitStore *rabbit.RabbitStore) *Context {
	return &Context{
		Rbt:              rabbitStore,
		Db:               c.Db,
		XHedgeForkBlock:  c.XHedgeForkBlock,
		ShaGateForkBlock: c.ShaGateForkBlock,
		Height:           c.Height,
	}
}

func (c *Context) WithDb(db modbtypes.DB) *Context {
	return &Context{
		Rbt:              c.Rbt,
		Db:               db,
		XHedgeForkBlock:  c.XHedgeForkBlock,
		ShaGateForkBlock: c.ShaGateForkBlock,
		Height:           c.Height,
	}
}

func (c *Context) SetXHedgeForkBlock(xHedgeForkBlock int64) {
	c.XHedgeForkBlock = xHedgeForkBlock
}

func (c *Context) SetShaGateForkBlock(shaGateForkBlock int64) {
	c.ShaGateForkBlock = shaGateForkBlock
}

func (c *Context) SetCurrentHeight(height int64) {
	c.Height = height
}

func (c *Context) IsXHedgeFork() bool {
	return c.Height >= c.XHedgeForkBlock
}

func (c *Context) IsShaGateFork() bool {
	return c.Height >= c.ShaGateForkBlock
}

//new empty rbt with same parent store as the old one
func (c *Context) WithRbtCopy() *Context {
	if !c.Rbt.IsClean() {
		panic("Can not copy when rabbitstore is not clean")
	}
	parent := c.Rbt.GetBaseStore()
	r := rabbit.NewRabbitStore(parent)
	return &Context{
		Rbt:              &r,
		Db:               c.Db,
		ShaGateForkBlock: c.ShaGateForkBlock,
		XHedgeForkBlock:  c.XHedgeForkBlock,
		Height:           c.Height,
	}
}

func (c *Context) Close(dirty bool) {
	if c.Rbt != nil {
		c.Rbt.CloseAndWriteBack(dirty)
	}
}

func (c *Context) GetAccount(address common.Address) *AccountInfo {
	k := GetAccountKey(address)
	v := c.Rbt.Get(k)
	if len(v) == 0 {
		return nil
	}
	return NewAccountInfo(v)
}

func (c *Context) SetAccount(address common.Address, acc *AccountInfo) {
	k := GetAccountKey(address)
	c.Rbt.Set(k, acc.Bytes())
}

func (c *Context) GetCode(contract common.Address) *BytecodeInfo {
	k := GetBytecodeKey(contract)
	v := c.Rbt.Get(k)
	if v != nil {
		return NewBytecodeInfo(v)
	}
	return nil
}

func (c *Context) GetStorageAt(seq uint64, key string) []byte {
	k := GetValueKey(seq, key)
	return c.Rbt.Get(k)
}

func (c *Context) GetValueAtMapKey(seq uint64, mapSlot string, mapKey string) []byte {
	key := crypto.Keccak256([]byte(mapKey), []byte(mapSlot))
	return c.GetStorageAt(seq, string(key))
}

func (c *Context) SetValueAtMapKey(seq uint64, mapSlot string, mapKey string, val []byte) {
	key := crypto.Keccak256([]byte(mapKey), []byte(mapSlot))
	c.SetStorageAt(seq, string(key), val)
}

func (c *Context) DeleteValueAtMapKey(seq uint64, mapSlot string, mapKey string) {
	key := crypto.Keccak256([]byte(mapKey), []byte(mapSlot))
	c.DeleteStorageAt(seq, string(key))
}

func (c *Context) GetAndDeleteValueAtMapKey(seq uint64, mapSlot string, mapKey string) []byte {
	key := crypto.Keccak256([]byte(mapKey), []byte(mapSlot))
	res := c.GetStorageAt(seq, string(key))
	c.DeleteStorageAt(seq, string(key))
	return res
}

func (c *Context) GetDynamicArray(seq uint64, arrSlot string) (res [][]byte) {
	arrLen := uint256.NewInt(0)
	arrLenBz := c.GetStorageAt(seq, arrSlot)
	if len(arrLenBz) == 32 {
		arrLen.SetBytes32(arrLenBz)
	}
	startSlot := uint256.NewInt(0).SetBytes32(crypto.Keccak256([]byte(arrSlot)))
	endSlot := uint256.NewInt(0).Add(startSlot, arrLen)
	for startSlot.Lt(endSlot) {
		res = append(res, c.GetStorageAt(seq, string(startSlot.Bytes())))
		startSlot.AddUint64(startSlot, 1)
	}
	return res
}

func (c *Context) CreateDynamicArray(seq uint64, arrSlot string, contents [][]byte) {
	arrLen := uint256.NewInt(uint64(len(contents)))
	c.SetStorageAt(seq, arrSlot, arrLen.PaddedBytes(32))
	startSlot := uint256.NewInt(0).SetBytes32(crypto.Keccak256([]byte(arrSlot)))
	for i, val := range contents {
		currSlot := uint256.NewInt(0).AddUint64(startSlot, uint64(i))
		c.SetStorageAt(seq, string(currSlot.PaddedBytes(32)), val)
	}
}

func (c *Context) DeleteDynamicArray(seq uint64, arrSlot string) {
	arrLen := uint256.NewInt(0)
	arrLenBz := c.GetStorageAt(seq, arrSlot)
	if len(arrLenBz) == 32 {
		arrLen.SetBytes32(arrLenBz)
	}
	startSlot := uint256.NewInt(0).SetBytes32(crypto.Keccak256([]byte(arrSlot)))
	endSlot := uint256.NewInt(0).Add(startSlot, arrLen)
	for startSlot.Lt(endSlot) {
		c.DeleteStorageAt(seq, string(startSlot.Bytes()))
		startSlot.AddUint64(startSlot, 1)
	}
	c.DeleteStorageAt(seq, arrSlot)
}

func (c *Context) SetStorageAt(seq uint64, key string, val []byte) {
	k := GetValueKey(seq, key)
	c.Rbt.Set(k, val)
}

func (c *Context) DeleteStorageAt(seq uint64, key string) {
	k := GetValueKey(seq, key)
	c.Rbt.Delete(k)
}

func (c *Context) GetCurrBlockBasicInfo() *Block {
	blk := &Block{}
	data := c.Rbt.Get([]byte{CURR_BLOCK_KEY})
	if len(data) == 0 {
		return nil
	}
	blk.FillBasicInfo(data)
	return blk
}

func (c *Context) SetCurrBlockBasicInfo(blk *Block) {
	c.Rbt.Set([]byte{CURR_BLOCK_KEY}, blk.SerializeBasicInfo())
}

func (c *Context) StoreBlock(blk *modbtypes.Block, txid2sigMap map[[32]byte][65]byte) {
	c.Db.AddBlock(blk, -1, txid2sigMap)
}

func (c *Context) GetLatestHeight() int64 {
	return c.Db.GetLatestHeight()
}

func (c *Context) GetTxByBlkHtAndTxIndex(height uint64, index uint64) *Transaction {
	bz := c.Db.GetTxByHeightAndIndex(int64(height), int(index))
	tx := &Transaction{}
	_, err := tx.UnmarshalMsg(bz)
	if err != nil {
		panic(err)
	}
	return tx
}

func (c *Context) GetTxByHash(txHash common.Hash) (tx *Transaction, sig [65]byte, err error) {
	c.Db.GetTxByHash(txHash, func(b []byte) bool {
		tmp := &Transaction{}
		_, err := tmp.UnmarshalMsg(b[65:])
		if err == nil && bytes.Equal(tmp.Hash[:], txHash[:]) {
			tx = tmp
			copy(sig[:], b[:65])
			return true // stop retry
		}
		return false
	})
	if tx == nil {
		err = ErrTxNotFound
	}
	return
}

func (c *Context) GetBlockHashByHeight(height uint64) [32]byte {
	var zero32 [32]byte
	res := c.Db.GetBlockHashByHeight(int64(height))
	if res == zero32 {
		blk, err := c.GetBlockByHeight(height)
		if err == nil {
			return blk.Hash
		}
	}
	return res
}

func (c *Context) GetBlockByHeight(height uint64) (*Block, error) {
	bz := c.Db.GetBlockByHeight(int64(height))
	if len(bz) == 0 {
		return nil, ErrBlockNotFound
	}

	blk := &Block{}
	_, err := blk.UnmarshalMsg(bz)
	if err != nil {
		return nil, err
	}
	return blk, nil
}

func (c *Context) GetBlockByHash(hash common.Hash) (blk *Block, err error) {
	c.Db.GetBlockByHash(hash, func(bz []byte) bool {
		tmp := &Block{}
		_, err := tmp.UnmarshalMsg(bz)
		if err == nil && bytes.Equal(hash[:], tmp.Hash[:]) {
			blk = tmp
			return true // stop retry
		}
		return false
	})
	if blk == nil {
		err = ErrBlockNotFound
	}
	return
}

func (c *Context) GetBalance(owner common.Address) (*uint256.Int, error) {
	if acc := c.GetAccount(owner); acc != nil {
		return acc.Balance(), nil
	}
	return nil, ErrAccNotFound
}

func (c *Context) CheckNonce(sender common.Address, nonce uint64) (*AccountInfo, error) {
	acc := c.GetAccount(sender)
	if acc == nil {
		return nil, ErrAccountNotExist
	}
	n := acc.Nonce()
	if nonce < n {
		return acc, ErrNonceTooSmall
	} else if nonce > n {
		return acc, ErrNonceTooLarge
	}
	return acc, nil
}

//func (c *Context) DeductTxFeeWithSpecificNonce(sender common.Address, acc *AccountInfo, txGas uint64, gasPrice *uint256.Int, newNonce uint64) error {
//	acc.UpdateNonce(newNonce)
//	var gasFee, gas uint256.Int
//	gas.SetUint64(txGas)
//	gasFee.Mul(&gas, gasPrice)
//	x := acc.Balance()
//	if x.Cmp(&gasFee) < 0 {
//		return errors.New("account balance is not enough for fee")
//	}
//	x.Sub(x, &gasFee)
//	acc.UpdateBalance(x)
//	c.SetAccount(sender, acc)
//	return nil
//}

func isInTopicSlice(topic [32]byte, topics [][32]byte) bool {
	for _, t := range topics {
		if bytes.Equal(t[:], topic[:]) {
			return true
		}
	}
	return false
}

func (c *Context) BasicQueryLogs(address common.Address, topics []common.Hash,
	startHeight, endHeight, limit uint32) (logs []Log, err error) {

	var rawAddress [20]byte = address
	rawTopics := FromGethHashes(topics)
	err = c.Db.BasicQueryLogs(&rawAddress, rawTopics, startHeight, endHeight, func(data []byte) (needMore bool) {
		if data == nil {
			err = ErrTooManyEntries
			return false
		}
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data[65:]); err != nil {
			return false
		}
		for _, log := range tx.Logs {
			hasAll := rawAddress == log.Address
			for _, t := range topics {
				if !hasAll {
					break
				}
				hasAll = hasAll && isInTopicSlice(t, log.Topics)
			}
			if hasAll {
				logs = append(logs, log)
				if limit > 0 && len(logs) >= int(limit) {
					return false
				}
			}
		}
		return true
	})
	return
}

type FilterFunc func(addr common.Address, topics []common.Hash, addrList []common.Address, topicsList [][]common.Hash) (ok bool)

func (c *Context) QueryLogs(addresses []common.Address, topics [][]common.Hash, startHeight, endHeight uint32, filter FilterFunc) (logs []Log, err error) {
	rawAddresses := FromGethAddreses(addresses)
	rawTopics := make([][][32]byte, len(topics))
	for i, t := range topics {
		rawTopics[i] = FromGethHashes(t)
	}

	err = c.Db.QueryLogs(rawAddresses, rawTopics, startHeight, endHeight, func(data []byte) bool {
		if data == nil {
			err = ErrTooManyEntries
			return false
		}
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data[65:]); err != nil {
			return false
		}

		var topicArr [4]common.Hash
		for _, log := range tx.Logs {
			for i, topic := range log.Topics {
				topicArr[i] = common.Hash(topic)
			}
			if filter(common.Address(log.Address), topicArr[:len(log.Topics)], addresses, topics) {
				logs = append(logs, log)
			}
		}
		return true
	})
	return
}

func (c *Context) QueryTxBySrc(addr common.Address, startHeight, endHeight, limit uint32) (txs []*Transaction, sigs [][65]byte, err error) {
	err = c.Db.QueryTxBySrc(addr, startHeight, endHeight, func(data []byte) bool {
		if data == nil {
			err = ErrTooManyEntries
			return false
		}
		var sig [65]byte
		copy(sig[:], data[:65])
		sigs = append(sigs, sig)
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data[65:]); err != nil {
			return false
		}
		if bytes.Equal(tx.From[:], addr[:]) { // compare them to prevent hash-conflict corner case
			txs = append(txs, &tx)
		}
		if limit > 0 && len(txs) >= int(limit) {
			return false
		}
		return true
	})
	return
}

func (c *Context) QueryTxByDst(addr common.Address, startHeight, endHeight, limit uint32) (txs []*Transaction, sigs [][65]byte, err error) {
	err = c.Db.QueryTxByDst(addr, startHeight, endHeight, func(data []byte) bool {
		if data == nil {
			err = ErrTooManyEntries
			return false
		}
		var sig [65]byte
		copy(sig[:], data[:65])
		sigs = append(sigs, sig)
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data[65:]); err != nil {
			return false
		}
		if bytes.Equal(tx.To[:], addr[:]) { // compare them to prevent hash-conflict corner case
			txs = append(txs, &tx)
		}
		if limit > 0 && len(txs) >= int(limit) {
			return false
		}
		return true
	})
	return
}

func (c *Context) QueryTxByAddr(addr common.Address, startHeight, endHeight, limit uint32) (txs []*Transaction, sigs [][65]byte, err error) {
	err = c.Db.QueryTxBySrcOrDst(addr, startHeight, endHeight, func(data []byte) bool {
		if data == nil {
			err = ErrTooManyEntries
			return false
		}
		var sig [65]byte
		copy(sig[:], data[:65])
		sigs = append(sigs, sig)
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data[65:]); err != nil {
			return false
		}
		if bytes.Equal(tx.From[:], addr[:]) || bytes.Equal(tx.To[:], addr[:]) {
			txs = append(txs, &tx)
		}
		if limit > 0 && len(txs) >= int(limit) {
			return false
		}
		return true
	})
	return
}

func (c *Context) GetTxListByHeight(height uint32) (txs []*Transaction, sigs [][65]byte, err error) {
	return c.GetTxListByHeightWithRange(height, 0, math.MaxInt32)
}

func (c *Context) GetTxListByHeightWithRange(height uint32, start, end int) (txs []*Transaction, sigs [][65]byte, err error) {
	txContents := c.Db.GetTxListByHeightWithRange(int64(height), start, end)
	txs = make([]*Transaction, len(txContents))
	sigs = make([][65]byte, len(txContents))
	for i, txContent := range txContents {
		copy(sigs[i][:], txContent[:65])
		txs[i] = &Transaction{}
		_, err = txs[i].UnmarshalMsg(txContent[65:])
		if err != nil {
			break
		}
	}
	return
}

// return the times addr acts as the to-address of a transaction
func (c *Context) GetToAddressCount(addr common.Address) int64 {
	k := append([]byte{modbtypes.TO_ADDR_KEY}, addr[:]...)
	return c.Db.QueryNotificationCounter(k)
}

// return the times addr acts as the from-address of a transaction
func (c *Context) GetFromAddressCount(addr common.Address) int64 {
	k := append([]byte{modbtypes.FROM_ADDR_KEY}, addr[:]...)
	return c.Db.QueryNotificationCounter(k)
}

// return the times addr acts as the to-address of a SEP20 Transfer event at some contract
func (c *Context) GetSep20ToAddressCount(contract common.Address, addr common.Address) int64 {
	var zero12 [12]byte
	k := append([]byte{modbtypes.TRANS_TO_ADDR_KEY}, contract[:]...)
	k = append(k, zero12[:]...)
	k = append(k, addr[:]...)
	return c.Db.QueryNotificationCounter(k)
}

// return the times addr acts as a from-address of a SEP20 Transfer event at some contract
func (c *Context) GetSep20FromAddressCount(contract common.Address, addr common.Address) int64 {
	var zero12 [12]byte
	k := append([]byte{modbtypes.TRANS_FROM_ADDR_KEY}, contract[:]...)
	k = append(k, zero12[:]...)
	k = append(k, addr[:]...)
	return c.Db.QueryNotificationCounter(k)
}

//	QueryTxBySrcOrDst(addr [20]byte, startHeight, endHeight uint32, fn func([]byte) bool) error
