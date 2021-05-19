package ebptests

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strings"

	"github.com/holiman/uint256"
	"github.com/smartbch/moeingads"
	"github.com/smartbch/moeingads/store"
	"github.com/smartbch/moeingads/store/rabbit"

	"github.com/smartbch/moeingevm/ebp"
	tc "github.com/smartbch/moeingevm/evmwrap/testcase"
	"github.com/smartbch/moeingevm/types"
	"github.com/smartbch/moeingevm/utils"
)

func WriteWorldStateToRabbit(rbt rabbit.RabbitStore, world *tc.WorldState) {
	for lsb, counter := range world.CreationCounters {
		k := types.GetCreationCounterKey(uint8(lsb))
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(counter))
		rbt.Set(k, buf[:])
	}
	for addr, bi := range world.Bytecodes {
		k := types.GetBytecodeKey(addr)
		bz := make([]byte, 32+len(bi.Bytecode))
		copy(bz[:32], bi.Codehash[:])
		copy(bz[32:], bi.Bytecode)
		rbt.Set(k, bz)
	}
	for addr, acc := range world.Accounts {
		k := types.GetAccountKey(addr)
		accInfo := types.ZeroAccountInfo()
		binary.BigEndian.PutUint64(accInfo.SequenceSlice(), acc.Sequence)
		accInfo.UpdateBalance(&acc.Balance)
		accInfo.UpdateNonce(acc.Nonce)
		rbt.Set(k, accInfo.Bytes())
	}
	for skey, bz := range world.Values {
		k := types.GetValueKey(skey.AccountSeq, string(skey.Key[:]))
		rbt.Set(k, bz)
	}
}

func UpdateWorldState(world *tc.WorldState, key, value []byte) {
	if key[0] == types.CREATION_COUNTER_KEY {
		world.CreationCounters[int(key[0])] = binary.BigEndian.Uint64(value)
	} else if key[0] == types.ACCOUNT_KEY {
		var addr [20]byte
		copy(addr[:], key[1:])
		accInfo := types.NewAccountInfo(value)
		world.Accounts[addr] = &tc.BasicAccount{
			Sequence: accInfo.Sequence(),
			Nonce:    accInfo.Nonce(),
		}
		world.Accounts[addr].Balance.SetBytes32(accInfo.BalanceSlice())
	} else if key[0] == types.BYTECODE_KEY {
		var addr [20]byte
		copy(addr[:], key[1:])
		bi := tc.BytecodeInfo{Bytecode: append([]byte{}, value[32:]...)}
		copy(bi.Codehash[:], key[:32])
		world.Bytecodes[addr] = bi
	} else if key[0] == types.VALUE_KEY {
		skey := tc.StorageKey{AccountSeq: binary.BigEndian.Uint64(key[1:9])}
		copy(skey.Key[:], key[9:])
		world.Values[skey] = append([]byte{}, value...)
	} else if bytes.Equal(types.StandbyTxQueueKey, key) {
		//Is OK
	} else if bytes.Equal([]byte{types.CURR_BLOCK_KEY}, key) {
		//Is OK
	} else {
		fmt.Printf("Why key %v value %v\n", key, value)
		panic("Unknown Key")
	}
}

var (
	GuardStart      = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	GuardStartPlus1 = []byte{0, 0, 0, 0, 0, 0, 0, 1}
	GuardEnd        = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
)

var IgnoreFiles []string

func runTestCase(filename string, theCase *tc.TestCase, printLog bool) {
	ebp.MaxTxGasLimit = math.MaxUint64
	for _, f := range IgnoreFiles {
		if strings.Contains(filename, f) {
			fmt.Printf("Ignore File: %s\n", filename)
			return
		}
	}
	fmt.Printf("NOW FILE %s\n", filename)
	if len(theCase.Blocks) != 1 {
		panic("not supported")
	}
	if len(theCase.Blocks[0].TxList) != 1 {
		panic("not supported")
	}
	world := &theCase.ImplState

	currBlock := theCase.Blocks[0]
	blockReward := uint256.NewInt().SetUint64(0)
	tc.AddBlockReward(world, currBlock.Coinbase, blockReward)

	// write tc.WorldState to MoeingADS
	os.RemoveAll("./rocksdb.db")
	mads := moeingads.NewMoeingADS4Mock([][]byte{GuardStart, GuardEnd})
	root := store.NewRootStore(mads, nil)
	defer root.Close()
	height := int64(1)
	root.SetHeight(height)
	trunk := root.GetTrunkStore().(*store.TrunkStore)
	rbt := rabbit.NewRabbitStore(trunk)
	WriteWorldStateToRabbit(rbt, world)
	rbt.Close()
	rbt.WriteBack()
	trunk.Close(true)

	// execute currTx
	currTx := currBlock.TxList[0].ToEthTx()
	trunk = root.GetTrunkStore().(*store.TrunkStore)
	var chainId big.Int
	chainId.SetBytes(currBlock.ChainId[:])
	txEngine := ebp.NewEbpTxExec(10, 100, 32, 100, &tc.DumbSigner{})
	ctx := types.NewContext(1, nil, nil)
	rbt = rabbit.NewRabbitStore(trunk)
	ctx = ctx.WithRbt(&rbt)
	txEngine.SetContext(ctx)
	txEngine.CollectTx(currTx)
	txEngine.Prepare(0, 0)
	txList := txEngine.CommittedTxs()
	fmt.Printf("after Prepare txList len %d\n", len(txList))
	txEngine.Execute(&currBlock.BlockInfo)
	trunk.Close(true)
	txList = txEngine.CommittedTxs()
	var gasFee uint256.Int
	gasFee.Mul(uint256.NewInt().SetUint64(txList[0].GasUsed),
		utils.U256FromSlice32(txList[0].GasPrice[:]))

	// create new tc.WorldState according to MoeingADS
	newWorld := tc.NewWorldState()
	world = &newWorld
	iter := mads.Iterator(GuardStartPlus1, GuardEnd)
	defer iter.Close()
	for iter.Valid() {
		key := iter.Key()
		value := iter.Value()
		if len(key) != 8 {
			panic(fmt.Sprintf("Strange Key %v", key))
		}
		cv := rabbit.BytesToCachedValue(value)
		UpdateWorldState(world, cv.GetKey(), cv.GetValue())
		iter.Next()
	}

	blockReward.SetUint64(2000000000000000000)
	blockReward.Add(blockReward, &gasFee)
	tc.AddBlockReward(world, currBlock.Coinbase, blockReward)

	foutRef, _ := os.Create("ref.txt")
	tc.PrintWorldState(foutRef, &theCase.RefState)
	foutRef.Close()

	foutImp, _ := os.Create("imp.txt")
	tc.PrintWorldState(foutImp, world)
	foutImp.Close()

	cmd := exec.Command("diff", "ref.txt", "imp.txt")
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL!! Compare %s %s\n", filename, theCase.Name)
		panic("FAIL")
	} else {
		fmt.Fprintf(os.Stderr, "PASS!! Compare %s %s\n", filename, theCase.Name)
	}

	os.RemoveAll("./rocksdb.db")
}

func InitIgnoreFiles() {
	IgnoreFiles = append(IgnoreFiles, "stAttackTest/CrashingTransaction.txt")
	IgnoreFiles = append(IgnoreFiles, "stBugs/randomStatetestDEFAULT-Tue_07_58_41-15153-575192.txt")
	IgnoreFiles = append(IgnoreFiles, "stEIP158Specific/vitalikTransactionTest.txt")
	IgnoreFiles = append(IgnoreFiles, "stSStoreTest/InitCollisionNonZeroNonce.txt")
	IgnoreFiles = append(IgnoreFiles, "stWalletTest")
	IgnoreFiles = append(IgnoreFiles, "stZeroKnowledge")
	IgnoreFiles = append(IgnoreFiles, "stSpecialTest/push32withoutByte.txt")
	IgnoreFiles = append(IgnoreFiles, "stSpecialTest/tx_e1c174e2.txt")
	IgnoreFiles = append(IgnoreFiles, "stRevertTest/RevertPrecompiledTouch")
	IgnoreFiles = append(IgnoreFiles, "stPreCompiledContracts/identity_to_bigger.txt")
	IgnoreFiles = append(IgnoreFiles, "stPreCompiledContracts/identity_to_smaller.txt")
	IgnoreFiles = append(IgnoreFiles, "stPreCompiledContracts/modexp_")
	IgnoreFiles = append(IgnoreFiles, "stPreCompiledContracts2/modexp_")
}

func StandaloneMain() {
	InitIgnoreFiles()
	args := os.Args
	me := args[0]
	fn := runTestCase
	if strings.HasSuffix(me, "run_test_case") {
		tc.RunOneCase(args, true, fn)
	} else if strings.HasSuffix(me, "run_test_file") {
		tc.RunOneFile(args, fn)
	} else if strings.HasSuffix(me, "run_test_dir") {
		tc.RunOneDir(args, true, fn)
	} else {
		fmt.Printf("NOT RUN \n")
	}
	//return
}
