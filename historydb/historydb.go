package historydb

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	it "github.com/smartbch/moeingads/indextree"
	adstypes "github.com/smartbch/moeingads/types"
	"github.com/smartbch/moeingdb/modb"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/smartbch/moeingevm/types"
)

const (
	CreationCounterByte = byte(100)
	AccountByte         = byte(102)
	BytecodeByte        = byte(104)
	StorageByte         = byte(106)

	Timeout = time.Second * 15
)

// You can use HistoricalRecord to send requests to RPC and check the result
// When key is 32-byte long, Value is StorageSlot
// When key is "account", Value is AccountInfo bytes
// When key is "bytecode", Value is BytecodeInfo bytes
type HistoricalRecord struct {
	Addr        [20]byte
	Key         string
	Value       []byte
	StartHeight uint64 //when this record was created
	EndHeight   uint64 //when this record was overwritten
}

type HistoryDb struct {
	rocksdb    *it.RocksDB
	batch      adstypes.Batch
	currHeight [8]byte
}

func NewHisDb(dirname string) *HistoryDb {
	db := &HistoryDb{}
	var err error
	db.rocksdb, err = it.NewRocksDB(dirname, ".")
	if err != nil {
		panic(err)
	}
	return db
}

func (db *HistoryDb) Close() {
	db.rocksdb.Close()
}

func (db *HistoryDb) BeginWrite(height uint64) {
	db.batch = db.rocksdb.NewBatch()
	binary.BigEndian.PutUint64(db.currHeight[:], height)
}

func (db *HistoryDb) EndWrite() {
	db.batch.WriteSync()
	db.batch.Close()
	db.batch = nil
}

func (db *HistoryDb) AddRwLists(height uint64, rwLists *types.ReadWriteLists) {
	db.BeginWrite(height)
	seq2Addr := make(map[uint64][20]byte, len(rwLists.AccountRList)+len(rwLists.AccountWList))
	for _, op := range rwLists.AccountRList {
		accInfo := types.NewAccountInfo(op.Account)
		seq2Addr[accInfo.Sequence()] = op.Addr
	}
	for _, op := range rwLists.AccountWList {
		accInfo := types.NewAccountInfo(op.Account)
		seq2Addr[accInfo.Sequence()] = op.Addr
	}
	for _, op := range rwLists.CreationCounterWList {
		var key [1 + 1 + 8]byte
		key[0] = CreationCounterByte
		key[1] = op.Lsb
		copy(key[2:], db.currHeight[:])
		var value [8]byte
		binary.LittleEndian.PutUint64(value[:], op.Counter)
		db.batch.Set(key[:], value[:])
	}
	for _, op := range rwLists.AccountWList {
		var key [1 + 20 + 8]byte
		key[0] = AccountByte
		copy(key[1:], op.Addr[:])
		copy(key[1+20:], db.currHeight[:])
		db.batch.Set(key[:], op.Account[:])
	}
	for _, op := range rwLists.BytecodeWList {
		var key [1 + 20 + 8]byte
		key[0] = BytecodeByte
		copy(key[1:], op.Addr[:])
		copy(key[1+20:], db.currHeight[:])
		db.batch.Set(key[:], op.Bytecode[:])
	}
	for _, op := range rwLists.StorageWList {
		var key [1 + 20 + 32 + 8]byte
		key[0] = StorageByte
		addr, ok := seq2Addr[op.Seq]
		if !ok {
			panic("Cannot find seq's addr")
		}
		copy(key[1:], addr[:])
		if len(op.Key) != 32 {
			panic("Invalid Key Length")
		}
		copy(key[1+20:], op.Key)
		copy(key[1+20+32:], db.currHeight[:])
		db.batch.Set(key[:], op.Value)
	}
	db.EndWrite()
}

func (db *HistoryDb) AddRwListAtHeight(ctx *types.Context, height uint64) {
	blk, err := ctx.GetBlockByHeight(height)
	if err != nil {
		panic(err)
	}
	for _, txHash := range blk.Transactions {
		tx, _, err := ctx.GetTxByHash(txHash)
		if err != nil {
			panic(err)
		}
		db.AddRwLists(height, tx.RwLists)
	}
}

func (db *HistoryDb) Fill(ctx *types.Context, endHeight uint64) {
	for h := uint64(0); h < endHeight; h++ {
		db.AddRwListAtHeight(ctx, h)
	}
}

//type HistoricalRecord struct {
//	Addr        [20]byte
//	Key         string
//	Value       []byte
//	StartHeight uint64 //when this record was created
//	EndHeight   uint64 //when this record was overwritten
func getRecord(key, value []byte) (rec HistoricalRecord) {
	if key[0] == AccountByte {
		copy(rec.Addr[:], key[1:])
		rec.Key = "account"
		rec.Value = append([]byte{}, value...)
		rec.StartHeight = binary.LittleEndian.Uint64(key[1+20:])
	} else if key[0] == BytecodeByte {
		copy(rec.Addr[:], key[1:])
		rec.Key = "bytecode"
		rec.Value = append([]byte{}, value...)
		rec.StartHeight = binary.LittleEndian.Uint64(key[1+20:])
	} else if key[0] == StorageByte {
		copy(rec.Addr[:], key[1:])
		rec.Key = string(key[1+20 : 1+20+32])
		rec.Value = append([]byte{}, value...)
		rec.StartHeight = binary.LittleEndian.Uint64(key[1+20+32:])
	} else {
		panic("invalid key[0]")
	}
	rec.EndHeight = math.MaxUint64
	return
}

func (db *HistoryDb) GenerateRecords(recChan chan HistoricalRecord) {
	iter := db.rocksdb.Iterator([]byte{AccountByte}, []byte{StorageByte + 1})
	defer iter.Close()
	if !iter.Valid() {
		return
	}
	currRec := getRecord(iter.Key(), iter.Value())
	for iter.Valid() {
		key, value := iter.Key(), iter.Value()
		if (currRec.Key == "account" && key[0] == AccountByte) ||
			(currRec.Key == "bytecode" && key[0] == StorageByte) {
			if bytes.Equal(currRec.Addr[:], key[1:1+20]) {
				currRec.EndHeight = binary.LittleEndian.Uint64(key[1+20:])
			}
		} else if len(currRec.Key) == 32 && key[0] == StorageByte {
			if bytes.Equal(currRec.Addr[:], key[1:1+20]) && currRec.Key == string(key[1+20:1+20+32]) {
				currRec.EndHeight = binary.LittleEndian.Uint64(key[1+20+32:])
			}
		}
		recChan <- currRec
		currRec = getRecord(key, value)
		iter.Next()
	}
	recChan <- currRec
}

// -------------------------------------------------------------------------------

func generateHisDb(modbDir, hisdbDir string, endHeight uint64) {
	modb := modb.NewMoDB(modbDir, log.NewNopLogger())
	ctx := types.NewContext(nil, modb)
	hisDb := NewHisDb(hisdbDir)
	hisDb.Fill(ctx, endHeight)
	hisDb.Close()
}

func getEthClient(rpcUrl string) *ethclient.Client {
	rpcCli, err := rpc.DialContext(context.Background(), rpcUrl)
	if err != nil {
		panic(err)
	}
	return ethclient.NewClient(rpcCli)
}

func runTestcases(hisdbDir, rpcUrl string) {
	ethCli := getEthClient(rpcUrl)
	recChan := make(chan HistoricalRecord, 100)
	hisDb := NewHisDb(hisdbDir)
	go hisDb.GenerateRecords(recChan)
	for rec := range recChan {
		mid := rand.Intn(int(rec.EndHeight)-int(rec.StartHeight)) + int(rec.StartHeight)
		if rec.Key == "account" {
			runAccountTestcase(rec, ethCli, rec.StartHeight)
			runAccountTestcase(rec, ethCli, uint64(mid))
			runAccountTestcase(rec, ethCli, rec.EndHeight-1)
		} else if rec.Key == "bytecode" {
			runBytecodeTestcase(rec, ethCli, rec.StartHeight)
			runBytecodeTestcase(rec, ethCli, uint64(mid))
			runBytecodeTestcase(rec, ethCli, rec.EndHeight-1)
		} else if len(rec.Key) == 32 {
			runStorageTestcase(rec, ethCli, rec.StartHeight)
			runStorageTestcase(rec, ethCli, uint64(mid))
			runStorageTestcase(rec, ethCli, rec.EndHeight-1)
		} else {
			panic("Invalid rec.Key")
		}
	}
}

func runAccountTestcase(rec HistoricalRecord, ethCli *ethclient.Client, height uint64) {
	h := big.NewInt(int64(height))
	accInfo := types.NewAccountInfo(rec.Value)
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	nonce, err := ethCli.NonceAt(ctx, common.Address(rec.Addr), h)
	if err != nil {
		panic(err)
	}
	if accInfo.Nonce() != nonce {
		fmt.Printf("height %d acc %s\n", height, common.Address(rec.Addr))
		fmt.Printf("nonce ref %d imp %d", accInfo.Nonce(), nonce)
	}
	balance, err := ethCli.BalanceAt(ctx, common.Address(rec.Addr), h)
	if err != nil {
		panic(err)
	}
	if accInfo.Balance().ToBig().Cmp(balance) != 0 {
		fmt.Printf("account %d acc %s\n", height, common.Address(rec.Addr))
		fmt.Printf("balance ref %s imp %s", accInfo.Balance(), balance)
	}
}

func runBytecodeTestcase(rec HistoricalRecord, ethCli *ethclient.Client, height uint64) {
	h := big.NewInt(int64(height))
	bcInfo := types.NewBytecodeInfo(rec.Value)
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	bytecode, err := ethCli.CodeAt(ctx, common.Address(rec.Addr), h)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(bcInfo.BytecodeSlice(), bytecode) {
		fmt.Printf("bytecode %d acc %s\n", height, common.Address(rec.Addr))
		fmt.Printf("ref %#v\n", bcInfo.BytecodeSlice())
		fmt.Printf("imp %#v\n", bytecode)
	}
}

func runStorageTestcase(rec HistoricalRecord, ethCli *ethclient.Client, height uint64) {
	h := big.NewInt(int64(height))
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	var key common.Hash
	copy(key[:], rec.Key)
	value, err := ethCli.StorageAt(ctx, common.Address(rec.Addr), key, h)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(rec.Value, value) {
		fmt.Printf("storage %d acc %s\n", height, common.Address(rec.Addr))
		fmt.Printf("ref %#v\n", rec.Value)
		fmt.Printf("imp %#v\n", value)
	}
}

func testTheOnlyTxInBlocks(modbDir, rpcUrl string, endHeight uint64) {
	modb := modb.NewMoDB(modbDir, log.NewNopLogger())
	ctx := types.NewContext(nil, modb)
	ethCli := getEthClient(rpcUrl)
	for h := uint64(0); h < endHeight; h++ {
		blk, err := ctx.GetBlockByHeight(h)
		if err != nil {
			panic(err)
		}
		if len(blk.Transactions) != 1 {
			continue
		}
		txHash := blk.Transactions[0]
		tx, _, err := ctx.GetTxByHash(txHash)
		if err != nil {
			panic(err)
		}
		testTheOnlyTx(tx, ethCli)
	}
}

func testTheOnlyTx(tx *types.Transaction, ethCli *ethclient.Client) {
	// TODO: check sbch_call using tx.RwLists
}
