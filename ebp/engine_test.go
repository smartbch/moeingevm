package ebp

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/smartbch/moeingads"
	"github.com/smartbch/moeingads/store"
	"github.com/smartbch/moeingads/store/rabbit"
	"github.com/smartbch/moeingdb/modb"

	"github.com/smartbch/moeingevm/evmwrap/testcase"
	"github.com/smartbch/moeingevm/types"
	//"github.com/smartbch/moeingevm/utils"
)

func prepareTruck() (*store.TrunkStore, *store.RootStore) {
	var (
		GuardStart = []byte{0, 0, 0, 0, 0, 0, 0, 0}
		GuardEnd   = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255}
	)
	mads, err := moeingads.NewMoeingADS("./testdbdata", false, [][]byte{GuardStart, GuardEnd})
	if err != nil {
		panic(err)
	}
	root := store.NewRootStore(mads, nil)
	height := int64(1)
	root.SetHeight(height)
	return root.GetTrunkStore(1000).(*store.TrunkStore), root
}

func prepareCtx(t *store.TrunkStore) *types.Context {
	rbt := rabbit.NewRabbitStore(t)
	return types.NewContext(&rbt, nil)
}

var (
	_, from1 = GenKeyAndAddr()
	from2    = common.HexToAddress("0x2")
	to1      = common.HexToAddress("0x10")
	to2      = common.HexToAddress("0x20")
)

func GenKeyAndAddr() (string, common.Address) {
	key, _ := gethcrypto.GenerateKey()
	keyHex := hex.EncodeToString(gethcrypto.FromECDSA(key))
	addr := gethcrypto.PubkeyToAddress(key.PublicKey)
	return keyHex, addr
}

func prepareAccAndTx(e *txEngine) []*gethtypes.Transaction {
	acc1 := types.ZeroAccountInfo()
	balance1, _ := uint256.FromBig(big.NewInt(10000_0000_0000))
	acc1.UpdateBalance(balance1)
	acc2 := types.ZeroAccountInfo()
	balance2, _ := uint256.FromBig(big.NewInt(10000_0000_0000))
	acc2.UpdateBalance(balance2)

	e.cleanCtx.SetAccount(from1, acc1)
	e.cleanCtx.SetAccount(from2, acc2)

	tx1, _ := gethtypes.NewTransaction(0, to1, big.NewInt(100), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx2, _ := gethtypes.NewTransaction(0, to2, big.NewInt(100), 100000, big.NewInt(1), nil).WithSignature(e.signer, from2.Bytes())
	e.cleanCtx.Close(true)
	return []*gethtypes.Transaction{tx1, tx2}
}

/*
testcase:
account1 send txs(nonce): 0
account2 send txs(nonce): 0
canCommitTxs: account1=>{0}; account2=>{0}
*/
func TestTxEngine_DifferentAccount(t *testing.T) {
	AdjustGasUsed = false
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	e := NewEbpTxExec(1, 100, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	txs := prepareAccAndTx(e)
	e.SetContext(prepareCtx(trunk))
	for _, tx := range txs {
		e.CollectTx(tx)
	}
	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	startKey, endKey := e.getStandbyQueueRange()
	txsStandby := e.loadStandbyTxs(&TxRange{
		start: startKey,
		end:   endKey,
	})
	require.Equal(t, 2, len(txsStandby))
	require.Equal(t, true, bytes.Equal(txs[0].To().Bytes(), txsStandby[0].To.Bytes()))
	require.Equal(t, true, bytes.Equal(txs[1].To().Bytes(), txsStandby[1].To.Bytes()))
	e.Execute(&types.BlockInfo{})
	require.Equal(t, 2, len(e.committedTxs))
	e.SetContext(prepareCtx(trunk))
	to1 := e.cleanCtx.GetAccount(*txs[0].To())
	to2 := e.cleanCtx.GetAccount(*txs[1].To())
	require.Equal(t, uint64(100), to1.Balance().Uint64())
	require.Equal(t, uint64(100), to2.Balance().Uint64())
	from1Acc := e.cleanCtx.GetAccount(from1)
	from2Acc := e.cleanCtx.GetAccount(from2)
	require.Equal(t, uint64(10000_0000_0000-21000-100), from1Acc.Balance().Uint64())
	require.Equal(t, uint64(10000_0000_0000-21000-100), from2Acc.Balance().Uint64())
	e.cleanCtx.Close(false)
	e.SetContext(prepareCtx(trunk))
	startKey, endKey = e.getStandbyQueueRange()
	require.Equal(t, true, startKey == endKey && endKey == 2)
}

/*
testcase:
account1 send txs(nonce): 0, 0, 2, 1, 2
account2 send txs(nonce): 0
canCommitTxs: account1=>{0,1,2}; account2=>{0}
*/
func TestTxEngine_SameAccount(t *testing.T) {
	AdjustGasUsed = false
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	e := NewEbpTxExec(5, 100, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	txs := prepareAccAndTx(e)
	e.SetContext(prepareCtx(trunk))
	for _, tx := range txs {
		e.CollectTx(tx)
	}
	//add more tx
	tx3, _ := gethtypes.NewTransaction(0, to1, big.NewInt(101), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx3)
	tx4, _ := gethtypes.NewTransaction(2, to1, big.NewInt(102), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx4)
	tx5, _ := gethtypes.NewTransaction(1, to1, big.NewInt(103), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx5)
	tx6, _ := gethtypes.NewTransaction(2, to1, big.NewInt(104), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx6)

	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	startKey, endKey := e.getStandbyQueueRange()
	txsStandby := e.loadStandbyTxs(&TxRange{
		start: startKey,
		end:   endKey,
	})
	require.Equal(t, 4, len(txsStandby))
	//require.Equal(t, true, bytes.Equal(txs[0].To().Bytes(), txsStandby[0].To.Bytes()))
	//require.Equal(t, true, bytes.Equal(txs[1].To().Bytes(), txsStandby[1].To.Bytes()))
	//require.Equal(t, true, bytes.Equal(utils.BigIntToSlice32(tx5.Value()), txsStandby[2].Value[:]))

	e.Execute(&types.BlockInfo{})
	require.Equal(t, 4, len(e.committedTxs))
	e.SetContext(prepareCtx(trunk))
	//check balance
	to1 := e.cleanCtx.GetAccount(*txs[0].To())
	to2 := e.cleanCtx.GetAccount(*txs[1].To())
	require.Equal(t, uint64(100+103+104), to1.Balance().Uint64())
	require.Equal(t, uint64(100), to2.Balance().Uint64())
	from1Acc := e.cleanCtx.GetAccount(from1)
	from2Acc := e.cleanCtx.GetAccount(from2)
	require.Equal(t, uint64(10000_0000_0000-21000-100-21000-103-21000-104), from1Acc.Balance().Uint64())
	require.Equal(t, uint64(10000_0000_0000-21000-100), from2Acc.Balance().Uint64())
	e.cleanCtx.Close(false)
	e.SetContext(prepareCtx(trunk))
	startKey, endKey = e.getStandbyQueueRange()
	//endKey:0=>4=>6=>7
	require.Equal(t, true, startKey == endKey && endKey == 7)
}

func generateRandomTx(s gethtypes.Signer) []*gethtypes.Transaction {
	rand.Seed(int64(time.Now().UnixNano()))
	set := make([]*gethtypes.Transaction, 2000)
	for i := 0; i < 1000; i++ {
		nonce := uint64(rand.Int() % 200)
		value := int64(rand.Int()%100 + 1)
		tx, _ := gethtypes.NewTransaction(nonce, to1, big.NewInt(value), 100000, big.NewInt(1), nil).WithSignature(s, from1.Bytes())
		set[i*2] = tx
		nonce = uint64(rand.Int() % 200)
		value = int64(rand.Int()%100 + 1)
		tx, _ = gethtypes.NewTransaction(nonce, to2, big.NewInt(value), 100000, big.NewInt(1), nil).WithSignature(s, from2.Bytes())
		set[i*2+1] = tx
	}
	return set
}

/*
random tx test
*/
func TestRandomTxExecuteConsistent(t *testing.T) {
	_, root := prepareTruck()
	defer closeTestCtx(root)
	randomTxs := generateRandomTx(&testcase.DumbSigner{})
	for i := 100; i > 0; i-- {
		r1 := executeTxs(randomTxs, root.GetTrunkStore(1000).(*store.TrunkStore))
		r2 := executeTxs(randomTxs, root.GetTrunkStore(1000).(*store.TrunkStore))
		//check txs
		require.Equal(t, len(r1.standbyTxs), len(r2.standbyTxs))
		for i, tx1 := range r1.standbyTxs {
			require.Equal(t, true, bytes.Equal(tx1.From.Bytes(), r2.standbyTxs[i].From.Bytes()))
			require.Equal(t, true, bytes.Equal(tx1.To.Bytes(), r2.standbyTxs[i].To.Bytes()))
			require.Equal(t, tx1.Nonce, r2.standbyTxs[i].Nonce)
			require.Equal(t, true, bytes.Equal(tx1.Value[:], r2.standbyTxs[i].Value[:]))
			fmt.Printf(
				`
from:%s
to:%s
nonce:%d
`, tx1.From.String(), tx1.To.String(), tx1.Nonce)
		}
		//check balance
		require.Equal(t, r1.from1.Balance(), r2.from1.Balance())
		require.Equal(t, r1.from2.Balance(), r2.from2.Balance())
		require.Equal(t, r1.to1.Balance(), r2.to1.Balance())
		require.Equal(t, r1.to2.Balance(), r2.to2.Balance())
		//check nonce
		require.Equal(t, r1.from1.Nonce(), r2.from1.Nonce())
		require.Equal(t, r1.from2.Nonce(), r2.from2.Nonce())
		require.Equal(t, r1.to1.Nonce(), r2.to1.Nonce())
		require.Equal(t, r1.to2.Nonce(), r2.to2.Nonce())
		//check standby tx
		require.Equal(t, r1.txR.start, r2.txR.start)
		require.Equal(t, r1.txR.end, r2.txR.end)
		//check committed tx
		require.Equal(t, len(r1.committedTxs), len(r2.committedTxs))
		require.Equal(t, len(r1.standbyTxs), len(r1.committedTxs))
		for i, tx1 := range r1.committedTxs {
			require.Equal(t, true, bytes.Equal(tx1.From[:], r2.committedTxs[i].From[:]))
			require.Equal(t, true, bytes.Equal(tx1.To[:], r2.committedTxs[i].To[:]))
			require.Equal(t, tx1.Nonce, r2.committedTxs[i].Nonce)
			require.Equal(t, true, bytes.Equal(tx1.Value[:], r2.committedTxs[i].Value[:]))
		}
	}
}

type executeResult struct {
	from1        *types.AccountInfo
	from2        *types.AccountInfo
	to1          *types.AccountInfo
	to2          *types.AccountInfo
	standbyTxs   []types.TxToRun
	committedTxs []*types.Transaction
	txR          *TxRange
}

func executeTxs(randomTxs []*gethtypes.Transaction, trunk *store.TrunkStore) executeResult {
	e := NewEbpTxExec(2000, 200, 30, 2000, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	_ = prepareAccAndTx(e)
	e.SetContext(prepareCtx(trunk))
	for _, tx := range randomTxs {
		e.CollectTx(tx)
	}
	e.Prepare(0, 0, DefaultTxGasLimit)
	startKey, endKey := e.getStandbyQueueRange()
	standbyTxs := e.loadStandbyTxs(&TxRange{
		start: startKey,
		end:   endKey,
	})
	e.SetContext(prepareCtx(trunk))
	e.Execute(&types.BlockInfo{})
	//collect states
	e.SetContext(prepareCtx(trunk))
	toAcc1 := e.cleanCtx.GetAccount(to1)
	toAcc2 := e.cleanCtx.GetAccount(to2)
	fromAcc1 := e.cleanCtx.GetAccount(from1)
	fromAcc2 := e.cleanCtx.GetAccount(from2)
	e.cleanCtx.Close(false)
	e.SetContext(prepareCtx(trunk))
	startKey, endKey = e.getStandbyQueueRange()
	t := TxRange{start: startKey, end: endKey}
	r := executeResult{
		from1:        fromAcc1,
		from2:        fromAcc2,
		to1:          toAcc1,
		to2:          toAcc2,
		txR:          &t,
		standbyTxs:   standbyTxs,
		committedTxs: e.committedTxs,
	}
	return r
}

func TestEmptyTxs(t *testing.T) {
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	e := NewEbpTxExec(5, 2, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	require.Equal(t, 0, e.CollectedTxsCount())
	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	e.Execute(&types.BlockInfo{})
	require.Equal(t, 0, len(e.CommittedTxs()))
}

func TestTxCountBiggerThanRunnerCount(t *testing.T) {
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	//only 1 runner
	e := NewEbpTxExec(5, 1, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	//2 tx
	txs := prepareAccAndTx(e)
	e.SetContext(prepareCtx(trunk))
	for _, tx := range txs {
		e.CollectTx(tx)
	}
	require.Equal(t, 2, e.CollectedTxsCount())
	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	e.Execute(&types.BlockInfo{})
	require.Equal(t, 2, len(e.CommittedTxs()))
}

func TestAccBalanceNotEnough(t *testing.T) {
	AdjustGasUsed = false
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	//only 1 runner
	e := NewEbpTxExec(5, 5, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	//2 tx
	txs := prepareAccAndTx(e)
	e.SetContext(prepareCtx(trunk))
	for _, tx := range txs {
		e.CollectTx(tx)
	}
	tx, _ := gethtypes.NewTransaction(1, to1, big.NewInt(20000_0000_0000), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx)
	require.Equal(t, 3, e.CollectedTxsCount())
	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	e.Execute(&types.BlockInfo{})
	toAcc1 := e.cleanCtx.GetAccount(to1)
	fromAcc1 := e.cleanCtx.GetAccount(from1)
	require.Equal(t, uint64(100), toAcc1.Balance().Uint64())
	require.Equal(t, uint64(10000_0000_0000-21000*2-100), fromAcc1.Balance().Uint64())
	//tx which account cannot pay for transfer value also can commit
	require.Equal(t, 3, len(e.CommittedTxs()))
	e.SetContext(prepareCtx(trunk))
}

func TestContractCreation(t *testing.T) {
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	//only 1 runner
	e := NewEbpTxExec(5, 5, 2, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	prepareAccAndTx(e)
	creationBytecode := hexToBytes(`
608060405234801561001057600080fd5b5060cc8061001f6000396000f3fe60
80604052348015600f57600080fd5b506004361060325760003560e01c806361
bc221a1460375780636299a6ef146053575b600080fd5b603d607e565b604051
8082815260200191505060405180910390f35b607c6004803603602081101560
6757600080fd5b81019080803590602001909291905050506084565b005b6000
5481565b8060008082825401925050819055505056fea2646970667358221220
37865cfcfd438966956583c78d31220c05c0f1ebfd116aced883214fcb1096c6
64736f6c634300060c0033
`)
	tx, _ := gethtypes.NewContractCreation(0, big.NewInt(0), 100000, big.NewInt(1), creationBytecode).WithSignature(e.signer, from1.Bytes())
	e.CollectTx(tx)
	e.SetContext(prepareCtx(trunk))
	e.Prepare(0, 0, DefaultTxGasLimit)
	e.SetContext(prepareCtx(trunk))
	e.Execute(&types.BlockInfo{})
	require.Equal(t, 1, len(e.committedTxs))
	contractAddr := gethcrypto.CreateAddress(from1, tx.Nonce())
	require.True(t, bytes.Equal(contractAddr[:], e.committedTxs[0].ContractAddress[:]))
}

func TestRandomPrepare(t *testing.T) {
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	e := NewEbpTxExec(5, 5, 5, 10, &testcase.DumbSigner{}, log.NewNopLogger())
	e.SetContext(prepareCtx(trunk))
	txs := prepareAccAndTx(e)
	e.CollectTx(txs[0])
	tx0, _ := gethtypes.NewTransaction(0, to1, big.NewInt(200), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx1, _ := gethtypes.NewTransaction(1, to1, big.NewInt(200), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx2, _ := gethtypes.NewTransaction(2, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx3, _ := gethtypes.NewTransaction(3, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx4, _ := gethtypes.NewTransaction(4, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx5, _ := gethtypes.NewTransaction(5, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx6, _ := gethtypes.NewTransaction(6, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx7, _ := gethtypes.NewTransaction(7, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx8, _ := gethtypes.NewTransaction(8, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx9, _ := gethtypes.NewTransaction(9, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx10, _ := gethtypes.NewTransaction(10, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())
	tx11, _ := gethtypes.NewTransaction(11, to1, big.NewInt(400), 100000, big.NewInt(1), nil).WithSignature(e.signer, from1.Bytes())

	for i := 0; i < 2; i++ {
		e.SetContext(prepareCtx(trunk))
		e.CollectTx(tx0)
		e.CollectTx(tx1)
		e.CollectTx(tx2)
		e.CollectTx(tx3)
		e.CollectTx(tx4)
		e.CollectTx(tx5)
		e.CollectTx(tx6)
		e.CollectTx(tx7)
		e.CollectTx(tx8)
		e.CollectTx(tx9)
		e.CollectTx(tx10)
		e.CollectTx(tx11)
		e.Prepare(0, 0, DefaultTxGasLimit)
		require.Equal(t, 12*i+12, e.StandbyQLen())
	}
}

func TestConsistentForDebug(t *testing.T) {
	trunk, root := prepareTruck()
	defer closeTestCtx(root)
	txs := make([]*gethtypes.Transaction, 9)
	signer := &testcase.DumbSigner{}
	tx, _ := gethtypes.NewTransaction(0, to2, big.NewInt(101), 100000, big.NewInt(1), nil).WithSignature(signer, from2.Bytes())
	txs[0] = tx
	tx, _ = gethtypes.NewTransaction(0, to1, big.NewInt(102), 100000, big.NewInt(1), nil).WithSignature(signer, from1.Bytes())
	txs[1] = tx
	tx, _ = gethtypes.NewTransaction(1, to2, big.NewInt(103), 100000, big.NewInt(1), nil).WithSignature(signer, from2.Bytes())
	txs[2] = tx
	tx, _ = gethtypes.NewTransaction(1, to1, big.NewInt(104), 100000, big.NewInt(1), nil).WithSignature(signer, from1.Bytes())
	txs[3] = tx
	tx, _ = gethtypes.NewTransaction(2, to2, big.NewInt(105), 100000, big.NewInt(1), nil).WithSignature(signer, from2.Bytes())
	txs[4] = tx
	tx, _ = gethtypes.NewTransaction(2, to1, big.NewInt(106), 100000, big.NewInt(1), nil).WithSignature(signer, from1.Bytes())
	txs[5] = tx
	tx, _ = gethtypes.NewTransaction(3, to1, big.NewInt(107), 100000, big.NewInt(1), nil).WithSignature(signer, from1.Bytes())
	txs[6] = tx
	tx, _ = gethtypes.NewTransaction(4, to1, big.NewInt(108), 100000, big.NewInt(1), nil).WithSignature(signer, from1.Bytes())
	txs[7] = tx
	tx, _ = gethtypes.NewTransaction(3, to2, big.NewInt(109), 100000, big.NewInt(1), nil).WithSignature(signer, from2.Bytes())
	txs[8] = tx
	r := executeTxs(txs, trunk)
	//fmt.Println(r.from1.Balance().Uint64())
	//fmt.Println(r.from2.Balance().Uint64())
	//fmt.Println(r.to1.Balance().Uint64())
	//fmt.Println(r.to2.Balance().Uint64())
	for _, tx := range r.standbyTxs {
		fmt.Printf(
			`
starndy tx:
from:%s
to:%s
nonce:%d
`, tx.From.String(), tx.To.String(), tx.Nonce)
	}
	for _, tx := range r.committedTxs {
		fmt.Printf(
			`
commited tx:
from:%v
to:%v
nonce:%d
`, tx.From, tx.To, tx.Nonce)
	}
}

func TestCcUtxoOpListsForMoDB(t *testing.T) {
	require.Equal(t, "0x15bab6fd59710de61ff75fa11875274a47fc2179068400add57ba8fb8bb4c5f1", NewRedeemable.Hex())
	require.Equal(t, "0x5097ba403df8e5415e49ecafe3a1610dce19fdae7df003d29d07d4f0833542ee", NewLostAndFound.Hex())
	require.Equal(t, "0x8a9c454bba797fa0dfd6fb9d59687e2e0d5e4828de1f91ffdcf4719e1163aec0", Redeem.Hex())
	require.Equal(t, "0xb2a43f090df954aa921e11f176ae7217b510ff0ea78dbb3d2bec4b06ad92a35a", ChangeAddr.Hex())
	require.Equal(t, "0x88efadfda2430f2d2ac267ce7158a19f80c4faef7beef319a98ba853e3ebed6f", Deleted.Hex())

	exec := &txEngine{}
	exec.committedTxs = []*types.Transaction{
		{
			Logs: []types.Log{
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{NewRedeemable},
					Data: hexToBytes("0x" +
						"44019a3ae9c861c6c805027b5150d768b03186a58269717972cec9dfcabc108e" +
						"000000000000000000000000000000000000000000000000000000000000007b" +
						"00000000000000000000000060d8666337c854686f2cf8a49b777c223b72fe34"),
				},
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{NewRedeemable},
					Data: hexToBytes("0x" +
						"5b90e7b5db50a11c3ceeafc59ef9d6317ed3d46f67b1b197037c531bd044d881" +
						"0000000000000000000000000000000000000000000000000000000000000141" +
						"0000000000000000000000002fe637ff51a517e6f29f5498b38a3f1d18af5dc7"),
				},
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{NewLostAndFound},
					Data: hexToBytes("0x" +
						"90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df07" +
						"00000000000000000000000000000000000000000000000000000000000001c8" +
						"000000000000000000000000c370743331b37d3c6d0ee798b3918f6561af2c92"),
				},
			},
		},
		{
			Logs: []types.Log{
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{Redeem},
					Data: hexToBytes("0x" +
						"44019a3ae9c861c6c805027b5150d768b03186a58269717972cec9dfcabc108e" +
						"000000000000000000000000000000000000000000000000000000000000007b" +
						"00000000000000000000000060d8666337c854686f2cf8a49b777c223b72fe34" +
						"0000000000000000000000000000000000000000000000000000000000000002"),
				},
			},
		},
		{
			Logs: []types.Log{
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{ChangeAddr},
					Data: hexToBytes("0x" +
						"90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df07" +
						"0000000000000000000000000000000000000000000000000000000000000123" +
						"00000000000000000000000060d8666337c854686f2cf8a49b777c223b72fe34" +
						"000000000000000000000000c370743331b37d3c6d0ee798b3918f6561af2c92"),
				},
			},
		},
		{
			Logs: []types.Log{
				{
					Address: CcContractAddr,
					Topics:  [][32]byte{Deleted},
					Data: hexToBytes("0x" +
						"90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df07" +
						"0000000000000000000000000000000000000000000000000000000000000456" +
						"0000000000000000000000002FE637Ff51A517E6f29F5498B38a3F1d18Af5DC7" +
						"0000000000000000000000000000000000000000000000000000000000000009"),
				},
			},
		},
	}
	ops := exec.CcUtxoOpListsForMoDB()
	require.Equal(t, modb.OpListsForCcUtxo{
		NewRedeemableOps: []modb.NewRedeemableOp{
			{
				UtxoId:       hexToUtxoId("0x44019a3ae9c861c6c805027b5150d768b03186a58269717972cec9dfcabc108e0000007b"),
				CovenantAddr: hexToAddr("0x60d8666337C854686F2CF8A49B777c223b72fe34"),
			},
			{
				UtxoId:       hexToUtxoId("0x5b90e7b5db50a11c3ceeafc59ef9d6317ed3d46f67b1b197037c531bd044d88100000141"),
				CovenantAddr: hexToAddr("0x2FE637Ff51A517E6f29F5498B38a3F1d18Af5DC7"),
			},
		},
		NewLostAndFoundOps: []modb.NewLostAndFoundOp{
			{
				UtxoId:       hexToUtxoId("0x90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df07000001c8"),
				CovenantAddr: hexToAddr("0xc370743331B37d3C6D0Ee798B3918f6561Af2C92"),
			},
		},
		RedeemOps: []modb.RedeemOp{
			{
				UtxoId:       hexToUtxoId("0x44019a3ae9c861c6c805027b5150d768b03186a58269717972cec9dfcabc108e0000007b"),
				CovenantAddr: hexToAddr("0x60d8666337C854686F2CF8A49B777c223b72fe34"),
				SourceType:   2,
			},
		},
		ChangeAddrOps: []modb.ChangeAddrOp{
			{
				UtxoId:          hexToUtxoId("0x90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df0700000123"),
				OldCovenantAddr: hexToAddr("0x60d8666337C854686F2CF8A49B777c223b72fe34"),
				NewCovenantAddr: hexToAddr("0xc370743331B37d3C6D0Ee798B3918f6561Af2C92"),
			},
		},
		DeletedOps: []modb.DeletedOp{
			{
				UtxoId:       hexToUtxoId("0x90dc60b63a50d461ef7aa5652566741cf7ba75b0a66e7d56a09461d7cf70df0700000456"),
				CovenantAddr: hexToAddr("0x2FE637Ff51A517E6f29F5498B38a3F1d18Af5DC7"),
				SourceType:   9,
			},
		},
	}, ops)
}

func closeTestCtx(rootStore *store.RootStore) {
	rootStore.Close()
	_ = os.RemoveAll("./testdbdata")
}

func hexToBytes(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")

	bytes, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bytes
}

func hexToUtxoId(s string) (utxoId [36]byte) {
	copy(utxoId[:], hexToBytes(s))
	return
}
func hexToAddr(s string) (addr [20]byte) {
	copy(addr[:], hexToBytes(s))
	return
}
