package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/ethereum/go-ethereum/common"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/moeing-chain/MoeingADS/store/rabbit"
	modbtypes "github.com/moeing-chain/MoeingDB/types"
)

var (
	ErrAccountNotExist error = errors.New("account does not exist")
	ErrNonceTooSmall   error = errors.New("tx nonce is smaller than the account nonce")
	ErrNonceTooLarge   error = errors.New("tx nonce is smaller than the account nonce")
)

type Context struct {
	Height uint64
	Rbt    *rabbit.RabbitStore
	Db     modbtypes.DB
}

func NewContext(height uint64, rbt *rabbit.RabbitStore, db modbtypes.DB) *Context {
	return &Context{
		Height: height,
		Rbt:    rbt,
		Db:     db,
	}
}

func (c *Context) WithRbt(rabbitStore *rabbit.RabbitStore) *Context {
	return &Context{
		Height: c.Height,
		Rbt:    rabbitStore,
		Db:     c.Db,
	}
}

func (c *Context) WithDb(db modbtypes.DB) *Context {
	return &Context{
		Height: c.Height,
		Rbt:    c.Rbt,
		Db:     db,
	}
}

//new empty rbt with same parent store as the old one
func (c *Context) WithRbtCopy() *Context {
	if !c.Rbt.IsClean() {
		panic("Can not copy when rabbitstore is not clean")
	}
	parent := c.Rbt.GetBaseStore()
	r := rabbit.NewRabbitStore(parent)
	return &Context{
		Height: c.Height,
		Rbt:    &r,
		Db:     c.Db,
	}
}

func (c *Context) Close(dirty bool) {
	c.Rbt.CloseAndWriteBack(dirty)
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

func (c *Context) DeleteAccount(address common.Address) {
	k := GetAccountKey(address)
	c.Rbt.Delete(k)
}

func (c *Context) GetCode(contract common.Address) *BytecodeInfo {
	k := GetBytecodeKey(contract)
	v := c.Rbt.Get(k)
	if v != nil {
		return NewBytecodeInfo(v)
	}
	return nil
}

func (c *Context) SetCode(contract common.Address, code *BytecodeInfo) {
	k := GetBytecodeKey(contract)
	c.Rbt.Set(k, code.Bytes())
}

func (c *Context) GetStorageAt(seq uint64, key string) []byte {
	k := GetValueKey(seq, key)
	return c.Rbt.Get(k)
}

func (c *Context) SetStorageAt(seq uint64, key string, val []byte) {
	k := GetValueKey(seq, key)
	c.Rbt.Set(k, val)
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

func (c *Context) GetCurrValidators() []ed25519.PubKey {
	bz := c.Rbt.Get([]byte{CURR_VALIDATORS_KEY})
	var vals []ed25519.PubKey
	if bz != nil {
		err := json.Unmarshal(bz, &vals)
		if err != nil {
			panic(err)
		}
	}
	return vals
}

func (c *Context) SetCurrValidators(vals []ed25519.PubKey) {
	b, err := json.Marshal(vals)
	if err != nil {
		panic(err)
	}
	c.Rbt.Set([]byte{CURR_VALIDATORS_KEY}, b)
}

func (c *Context) StoreBlock(blk *modbtypes.Block) {
	c.Db.AddBlock(blk, -1)
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

func (c *Context) GetTxByHash(txHash common.Hash) (tx *Transaction, err error) {
	c.Db.GetTxByHash(txHash, func(b []byte) bool {
		tmp := &Transaction{}
		_, err := tmp.UnmarshalMsg(b)
		if err == nil && bytes.Equal(tmp.Hash[:], txHash[:]) {
			tx = tmp
			return true // stop retry
		}
		return true
	})
	if tx == nil {
		err = ErrTxNotFound
	}
	return
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

func (c *Context) GetBalance(owner common.Address, height int64) (*uint256.Int, error) {
	if height != int64(gethrpc.LatestBlockNumber) {
		return nil, fmt.Errorf("TODO: GetBalance(), h=%d", height)
	}

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
	//fmt.Printf("acc:%s, acc nonce:%d, tx nonce:%d\n", sender, n, nonce)
	if nonce < n {
		return nil, ErrNonceTooSmall
	} else if nonce > n {
		return nil, ErrNonceTooLarge
	}
	return acc, nil
}

func (c *Context) IncrNonce(sender common.Address, acc *AccountInfo) {
	acc.UpdateNonce(acc.Nonce() + 1)
	c.SetAccount(sender, acc)
}

func (c *Context) DeductTxFee(sender common.Address, acc *AccountInfo, txGas uint64, gasPrice *uint256.Int) error {
	acc.UpdateNonce(acc.Nonce() + 1)
	var gasFee, gas uint256.Int
	gas.SetUint64(txGas)
	gasFee.Mul(&gas, gasPrice)
	x := acc.Balance()
	x.Sub(x, &gasFee)
	if x.Cmp(acc.Balance()) == 1 {
		return errors.New("account balance is not enough for fee")
	}
	acc.UpdateBalance(x)
	c.SetAccount(sender, acc)
	return nil
}

func (c *Context) QueryLogs(addresses []common.Address, topics [][]common.Hash, startHeight, endHeight uint32) (logs []Log, err error) {
	rawAddresses := FromGethAddreses(addresses)
	rawTopics := make([][][32]byte, len(topics))
	for i, t := range topics {
		rawTopics[i] = FromGethHashes(t)
	}

	c.Db.QueryLogs(rawAddresses, rawTopics, startHeight, endHeight, func(data []byte) bool {
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data); err != nil {
			return false
		}

		logs = append(logs, tx.Logs...)
		return true
	})

	return
}

func (c *Context) QueryTxBySrc(addr common.Address, startHeight, endHeight uint32) (txs []*Transaction, err error) {
	c.Db.QueryTxBySrc(addr, startHeight, endHeight, func(data []byte) bool {
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data); err != nil {
			return false
		}
		txs = append(txs, &tx)
		return true
	})
	return
}

func (c *Context) QueryTxByDst(addr common.Address, startHeight, endHeight uint32) (txs []*Transaction, err error) {
	c.Db.QueryTxByDst(addr, startHeight, endHeight, func(data []byte) bool {
		tx := Transaction{}
		if _, err = tx.UnmarshalMsg(data); err != nil {
			return false
		}
		txs = append(txs, &tx)
		return true
	})
	return
}
