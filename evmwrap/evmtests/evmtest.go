package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	tc "github.com/smartbch/moeingevm/evmwrap/testcase"
)

/*
#cgo linux LDFLAGS: -l:libevmwrap.a -L../host_bridge -lstdc++
#cgo darwin LDFLAGS: -levmwrap -L../host_bridge -lstdc++
#include "../host_bridge/bridge.h"
int64_t zero_depth_call_wrap(evmc_bytes32 gas_price,
                     int64_t gas_limit,
                     const evmc_address* destination,
                     const evmc_address* sender,
                     const evmc_bytes32* value,
                     const uint8_t* input_data,
                     size_t input_size,
		     const struct block_info* block,
		     int handler,
		     bool need_gas_estimation,
                     enum evmc_revision revision,
		     bridge_query_executor_fn query_executor_fn);
*/
import "C"

//type bytes_info = C.struct_bytes_info
type evmc_address = C.struct_evmc_address
type evmc_bytes32 = C.struct_evmc_bytes32
type evmc_result = C.struct_evmc_result
type changed_account = C.struct_changed_account
type changed_creation_counter = C.struct_changed_creation_counter
type changed_bytecode = C.struct_changed_bytecode
type changed_value = C.struct_changed_value
type added_log = C.struct_added_log
type all_changed = C.struct_all_changed
type block_info = C.struct_block_info
type big_buffer = C.struct_big_buffer
type small_buffer = C.struct_small_buffer

func NewByteSlice(data *C.char, size C.int) []byte {
	bs := make([]byte, size)
	copy(bs, C.GoStringN(data, size))
	return bs
}

func NewBytecodeInfo(data *C.char, size C.int, hash *evmc_bytes32) (result tc.BytecodeInfo) {
	for i := range result.Codehash {
		result.Codehash[i] = byte(hash.bytes[i])
	}
	result.Bytecode = NewByteSlice(data, size)
	return
}

func toArr20(addr *evmc_address) [20]byte {
	var arr [20]byte
	for i := range arr {
		arr[i] = byte(addr.bytes[i])
	}
	return arr
}

func toArr32(addr *evmc_bytes32) [32]byte {
	var arr [32]byte
	for i := 0; i < 32; i++ {
		arr[i] = byte(addr.bytes[i])
	}
	return arr
}

func writeCBytes32WithBytes32(b32 *evmc_bytes32, bz []byte) {
	for i := 0; i < 32; i++ {
		b32.bytes[i] = C.uint8_t(bz[i])
	}
}

func writeCBytes32WithUInt256(b32 *evmc_bytes32, v *uint256.Int) {
	var bz [32]byte
	v.WriteToArray32(&bz)
	writeCBytes32WithBytes32(b32, bz[:])
}

func writeCBytes20WithArray(arr *evmc_address, bz [20]byte) {
	for i := 0; i < 20; i++ {
		arr.bytes[i] = C.uint8_t(bz[i])
	}
}

type ResultCollector struct {
	state     *tc.WorldState
	currTx    *tc.Tx
	currBlock *tc.TestBlock
	fout      io.Writer
	succeed   bool
	gasLeft   uint64
}

var WORLD *tc.WorldState
var COLLECTOR *ResultCollector

//export get_creation_counter
func get_creation_counter(handler C.int /*not used*/, n uint8) uint64 {
	return WORLD.CreationCounters[n]
}

//export get_account_info
func get_account_info(handler C.int /*not used*/, addr *evmc_address, balance *evmc_bytes32, nonce *C.uint64_t, sequence *C.uint64_t) {
	addrBytes := toArr20(addr)
	acc, ok := WORLD.Accounts[addrBytes]
	if !ok {
		*nonce = ^C.uint64_t(0)
		return
	}

	writeCBytes32WithUInt256(balance, &acc.Balance)
	*nonce = C.uint64_t(acc.Nonce)
	*sequence = C.uint64_t(acc.Sequence)
}

//export get_bytecode
func get_bytecode(handler C.int /*not used*/, addr *evmc_address, codehash_ptr *evmc_bytes32, buf *big_buffer, size *C.size_t) {
	addrBytes := toArr20(addr)
	info, ok := WORLD.Bytecodes[addrBytes]
	if !ok {
		*size = 0
		return
	}
	for i := range info.Codehash {
		codehash_ptr.bytes[i] = C.uint8_t(info.Codehash[i])
	}
	for i := range info.Bytecode {
		buf.data[i] = C.uint8_t(info.Bytecode[i])
	}
	*size = C.size_t(len(info.Bytecode))
}

//export get_value
func get_value(handler C.int /*not used*/, acc_seq C.uint64_t, key_ptr *C.char, buf *big_buffer, size *C.size_t) {
	skey := NewStorageKey(acc_seq, key_ptr)
	v, ok := WORLD.Values[skey]
	if !ok {
		*size = 0
		return
	}
	for i := range v {
		buf.data[i] = C.uint8_t(v[i])
	}
	*size = C.size_t(len(v))
}

func NewStorageKey(acc_seq C.uint64_t, key_ptr *C.char) tc.StorageKey {
	skey := tc.StorageKey{AccountSeq: uint64(acc_seq)}
	copy(skey.Key[:], C.GoStringN(key_ptr, 32))
	return skey
}

//export get_block_hash
func get_block_hash(handler C.int /*not used*/, num C.uint64_t) (result evmc_bytes32) {
	n := uint64(num % 512)
	for i := range WORLD.BlockHashes[n] {
		result.bytes[i] = C.uint8_t(WORLD.BlockHashes[n][i])
	}
	return
}

// ==============================================

func (collector *ResultCollector) changeAccount(chg_acc *changed_account) {
	addrBytes := toArr20(chg_acc.address)
	if chg_acc.delete_me {
		if !bytes.Equal(collector.currBlock.Coinbase[:], addrBytes[:]) {
			delete(collector.state.Accounts, addrBytes)
		}
	} else {
		ba := &tc.BasicAccount{
			Nonce:    uint64(chg_acc.nonce),
			Sequence: uint64(chg_acc.sequence),
		}
		arr32 := toArr32(&chg_acc.balance)
		ba.Balance.SetBytes(arr32[:])
		collector.state.Accounts[addrBytes] = ba
	}
}

func (collector *ResultCollector) changeCreationCounter(chg_counter *changed_creation_counter) {
	collector.state.CreationCounters[chg_counter.lsb] = uint64(chg_counter.counter)
}

func (collector *ResultCollector) changeBytecode(chg_bytecode *changed_bytecode) {
	addrBytes := toArr20(chg_bytecode.address)
	if chg_bytecode.bytecode_size == 0 {
		delete(collector.state.Bytecodes, addrBytes)
	} else {
		bi := NewBytecodeInfo(chg_bytecode.bytecode_data, chg_bytecode.bytecode_size, chg_bytecode.codehash)
		collector.state.Bytecodes[addrBytes] = bi
	}
}

func (collector *ResultCollector) changeValue(chg_value *changed_value) {
	k := NewStorageKey(chg_value.account_seq, chg_value.key_ptr)
	if chg_value.value_size == 0 {
		delete(collector.state.Values, k)
	} else {
		collector.state.Values[k] = NewByteSlice(chg_value.value_data, chg_value.value_size)
	}
}

func (collector *ResultCollector) deductGasFee() {
	var gasFeeMax uint256.Int
	gasFeeMax.Mul(uint256.NewInt(0).SetUint64(collector.currTx.Gas), &collector.currTx.GasPrice)
	accounts := collector.state.Accounts
	x := &uint256.Int{}
	x.Set(&accounts[collector.currTx.From].Balance)
	x.Sub(x, &gasFeeMax)
	accounts[collector.currTx.From].Balance.Set(x)
}

func (collector *ResultCollector) refundGasFee(ret_value *evmc_result, refund C.uint64_t) {
	gasUsed := collector.currTx.Gas - uint64(ret_value.gas_left)
	//fmt.Printf("@GAS gas_left %d currTx.Gas %d gasUsed %d\n", uint64(ret_value.gas_left), collector.currTx.Gas, gasUsed)
	collector.gasLeft = uint64(ret_value.gas_left)
	half := (gasUsed + 1) / 2
	if uint64(refund) > half {
		gasUsed = half
	} else {
		gasUsed = gasUsed - uint64(refund)
	}

	accounts := collector.state.Accounts

	var returnedGasFee uint256.Int
	returnedGasFee.Mul(uint256.NewInt(0).SetUint64(collector.currTx.Gas-gasUsed), &collector.currTx.GasPrice)
	x := &uint256.Int{}
	x.Set(&accounts[collector.currTx.From].Balance)
	x.Add(x, &returnedGasFee)
	accounts[collector.currTx.From].Balance.Set(x)

	var gasFee uint256.Int
	gasFee.Mul(uint256.NewInt(0).SetUint64(gasUsed), &collector.currTx.GasPrice)
	x.Set(&accounts[collector.currBlock.Coinbase].Balance)
	x.Add(x, &gasFee)
	accounts[collector.currBlock.Coinbase].Balance.Set(x)
}

func PrintLog(fout io.Writer, log *added_log) {
	var topics [][32]byte
	if log.topic1 != nil {
		topics = append(topics, toArr32(log.topic1))
	}
	if log.topic2 != nil {
		topics = append(topics, toArr32(log.topic2))
	}
	if log.topic3 != nil {
		topics = append(topics, toArr32(log.topic3))
	}
	if log.topic4 != nil {
		topics = append(topics, toArr32(log.topic4))
	}
	fmt.Fprintf(fout, "LOG_ENTRY %d", len(topics))
	for _, topic := range topics {
		fmt.Fprintf(fout, " %s", common.BytesToHash(topic[:]).Hex())
	}
	addr := toArr20(log.contract_addr)
	data := C.GoStringN(log.data, log.size)
	fmt.Fprintf(fout, " %s %d", common.BytesToAddress(addr[:]).Hex(), len(data))
	for i := range data {
		fmt.Fprintf(fout, " %d", uint(data[i]))
	}
	fmt.Fprintf(fout, "\n")
}

//	fmt.Printf("LOG_COUNT %d\n", len(logs))
//	for _, log := range logs {
//		fmt.Printf("LOG_ENTRY %d", len(log.Topics))
//		for _, topic := range log.Topics {
//			fmt.Printf(" %s", topic.Hex())
//		}
//		fmt.Printf(" %s %d", log.Address.Hex(), len(log.Data))
//		for _, d := range log.Data {
//			fmt.Printf(" %d", d)
//		}
//		fmt.Printf("\n")
//	}

//export collect_result
func collect_result(handler C.int /*not used*/, result *all_changed, ret_value *evmc_result) {
	COLLECTOR.succeed = ret_value.status_code == C.EVMC_SUCCESS
	if result == nil {
		COLLECTOR.refundGasFee(ret_value, 0)
		return
	}
	//fmt.Printf("Why %#v\n", result)
	size := int(result.account_num)
	if size != 0 {
		accounts := (*[1 << 30]changed_account)(unsafe.Pointer(result.accounts))[:size:size]
		for _, elem := range accounts {
			COLLECTOR.changeAccount(&elem)
		}
	}
	size = int(result.creation_counter_num)
	if size != 0 {
		creation_counters := (*[1 << 30]changed_creation_counter)(unsafe.Pointer(result.creation_counters))[:size:size]
		for _, elem := range creation_counters {
			COLLECTOR.changeCreationCounter(&elem)
		}
	}
	size = int(result.bytecode_num)
	if size != 0 {
		bytecodes := (*[1 << 30]changed_bytecode)(unsafe.Pointer(result.bytecodes))[:size:size]
		for _, elem := range bytecodes {
			COLLECTOR.changeBytecode(&elem)
		}
	}
	size = int(result.value_num)
	if size != 0 {
		values := (*[1 << 30]changed_value)(unsafe.Pointer(result.values))[:size:size]
		for _, elem := range values {
			COLLECTOR.changeValue(&elem)
		}
	}
	size = int(result.log_num)
	if size != 0 && COLLECTOR.fout != nil {
		fmt.Fprintf(COLLECTOR.fout, "LOG_COUNT %d\n", size)
		logs := (*[1 << 30]added_log)(unsafe.Pointer(result.logs))[:size:size]
		for _, elem := range logs {
			PrintLog(COLLECTOR.fout, &elem)
		}
	} else if COLLECTOR.fout != nil {
		fmt.Fprintf(COLLECTOR.fout, "LOG_COUNT 0\n")
	}
	COLLECTOR.refundGasFee(ret_value, result.refund)
}

// ============================================================

const (
	ESTIMATE_GAS     = 1
	CHECK_GAS        = 2
	PRINT_POST_STATE = 3
)

func updateQueryExecutorFn() {
	aotDir := os.Getenv("AOTDIR")
	if len(aotDir) == 0 {
		return
	}
	if onlyReload := os.Getenv("ONLY_RELOAD"); len(onlyReload) != 0 {
		ReloadQueryExecutorFn(path.Join(aotDir, "out"))
		return
	}
	if err := os.RemoveAll(path.Join(aotDir, "in")); err != nil {
		panic(err)
	}
	if err := os.RemoveAll(path.Join(aotDir, "out")); err != nil {
		panic(err)
	}
	if err := os.Mkdir(path.Join(aotDir, "in"), 0750); err != nil {
		panic(err)
	}
	if err := os.Mkdir(path.Join(aotDir, "out"), 0750); err != nil {
		panic(err)
	}
	totalContracts := 0
	for addr, info := range WORLD.Bytecodes {
		if len(info.Bytecode) == 0 || len(info.Bytecode) > 24*1024 {
			continue
		}
		totalContracts++
		addrHex := hex.EncodeToString(addr[:])
		bytecodeHex := hex.EncodeToString(info.Bytecode[:])
		if err := ioutil.WriteFile(path.Join(aotDir, "in", addrHex), []byte(bytecodeHex), 0644); err != nil {
			panic(err)
		}
	}
	fmt.Println("Num of compiled contracts", totalContracts)
	if totalContracts == 0 {
		return
	}
	cmd := exec.Command("runaot", "gen", path.Join(aotDir, "in"), path.Join(aotDir, "out"))
	output, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("runaot output\n%s\n", string(output))

	cmd = exec.Command("bash", "compile.sh")
	cmd.Dir = path.Join(aotDir, "out")
	output, err = cmd.Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("compile output\n%s\n", string(output))

	if err != nil {
		panic(err)
	}
	ReloadQueryExecutorFn(path.Join(aotDir, "out"))
}

//nolint:deadcode
func runTestCaseSingle(filename string, theCase *tc.TestCase, printLog bool) {
	//ReloadQueryExecutorFn(path.Join(os.Getenv("AOTDIR"), "out"))
	runTestCaseWithGasLimit(filename, theCase, printLog, -1, ESTIMATE_GAS)
}

//nolint:deadcode
func runTestCaseDual(filename string, theCase *tc.TestCase, printLog bool) {
	copiedCase := &tc.TestCase{
		Name:      theCase.Name,
		ImplState: theCase.ImplState.Clone(),
		RefState:  theCase.RefState.Clone(),
		Blocks:    theCase.Blocks,
	}
	estimatedGas := runTestCaseWithGasLimit(filename, theCase, printLog, -1, ESTIMATE_GAS)
	fmt.Printf("estimatedGas %d\n", estimatedGas)
	if estimatedGas < 0 {
		panic("Error during estimation")
	} else if estimatedGas > 0 {
		runTestCaseWithGasLimit(filename, copiedCase, printLog, estimatedGas, CHECK_GAS)
	}
}

func runTestCaseWithGasLimit(filename string, theCase *tc.TestCase, printLog bool, gasLimit int64, mode int) int64 {
	if len(theCase.Blocks) != 1 {
		panic("not supported")
	}
	if len(theCase.Blocks[0].TxList) != 1 {
		panic("not supported")
	}
	currBlock := theCase.Blocks[0]
	currTx := currBlock.TxList[0]
	WORLD = &theCase.ImplState
	var blockReward uint256.Int
	blockReward.SetUint64(0)
	tc.AddBlockReward(WORLD, currBlock.Coinbase, &blockReward)

	if gasLimit > 0 {
		currTx.Gas = uint64(gasLimit)
	}

	COLLECTOR = &ResultCollector{
		state:     WORLD,
		currTx:    currTx,
		currBlock: currBlock,
	}
	if printLog {
		fout, _ := os.Create(filename[:len(filename)-4] + ".log")
		defer fout.Close()
		COLLECTOR.fout = fout
	}
	WORLD.Accounts[currTx.From].Nonce++
	COLLECTOR.deductGasFee()
	var value, gas_price evmc_bytes32
	var to, from evmc_address
	writeCBytes32WithUInt256(&value, &currTx.Value)
	writeCBytes32WithUInt256(&gas_price, &currTx.GasPrice)
	writeCBytes20WithArray(&to, currTx.To)
	writeCBytes20WithArray(&from, currTx.From)

	var bi block_info
	writeCBytes20WithArray(&bi.coinbase, currBlock.Coinbase)
	bi.number = C.int64_t(currBlock.Number)
	bi.timestamp = C.int64_t(currBlock.Timestamp)
	bi.gas_limit = C.int64_t(currBlock.GasLimit)
	bi.cfg.after_xhedge_fork = false
	writeCBytes32WithBytes32(&bi.difficulty, currBlock.Difficulty[:])
	writeCBytes32WithBytes32(&bi.chain_id, currBlock.ChainId[:])
	data_ptr := (*C.uint8_t)(nil)
	if len(currTx.Data) != 0 {
		data_ptr = (*C.uint8_t)(unsafe.Pointer(&currTx.Data[0]))
	}

	updateQueryExecutorFn()

	estimatedGas := C.zero_depth_call_wrap(gas_price,
		C.int64_t(currTx.Gas),
		&to,
		&from,
		&value,
		data_ptr,
		C.size_t(len(currTx.Data)),
		&bi,
		0,
		C.bool(mode == ESTIMATE_GAS),
		C.EVMC_ISTANBUL,
		QueryExecutorFn)

	blockReward.SetUint64(2000000000000000000)
	tc.AddBlockReward(WORLD, currBlock.Coinbase, &blockReward)

	if mode == CHECK_GAS {
		//fmt.Printf("WE_ESTIMATE %d LEFT %d %s %s\n", gasLimit, COLLECTOR.gasLeft, filename, theCase.Name)
		if !COLLECTOR.succeed {
			panic("status_code != EVMC_SUCCESS")
		}
	} else if mode == PRINT_POST_STATE {
		var addr [20]byte
		_, err := hex.Decode(addr[:], []byte("6295ee1b4f6dd65047762f924ecd367c17eabf8f"))
		if err != nil {
			panic(err)
		}
		fmt.Println(hex.EncodeToString(theCase.ImplState.Bytecodes[addr].Bytecode))
	} else {
		foutRef, _ := os.Create("ref.txt")
		tc.PrintWorldState(foutRef, &theCase.RefState)
		foutRef.Close()

		foutImp, _ := os.Create("imp.txt")
		tc.PrintWorldState(foutImp, &theCase.ImplState)
		foutImp.Close()

		cmd := exec.Command("diff", "ref.txt", "imp.txt")
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL!! Compare %s %s\n", filename, theCase.Name)
			panic("FAIL")
		} else {
			fmt.Fprintf(os.Stderr, "PASS!! Compare %s %s\n", filename, theCase.Name)
		}
	}
	WORLD = nil
	COLLECTOR = nil
	return int64(estimatedGas)
}

const deployCodeTestCase = `test deployCodeTestCase
pre
 addr 0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b
  acc_nonce 0
  balance 1000000000000
  code 0
blocks
 block 0
  coinbase 0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba
  height 1
  difficulty 131072
  gaslimit 20019530
  timestamp 1000
  tx 0
   from 0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b
   nonce 0
   to 0x0000000000000000000000000000000000000000
   value 0
   gasprice 1
   gas 15000000
   data 0
`

func getDeployedCode() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	bytecodeHex := scanner.Text()
	data, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		panic(err)
	}
	cases := tc.ReadTestCasesFromString(deployCodeTestCase)
	theCase := &cases[0]
	theCase.Blocks[0].TxList[0].Data = data
	runTestCaseWithGasLimit("", theCase, false, 9000*10000, PRINT_POST_STATE)
}

func main() {
	args := os.Args
	me := args[0]
	fn := runTestCaseSingle // runTestCaseSingle or runTestCaseDual
	if strings.HasSuffix(me, "run_test_case") {
		tc.RunOneCase(args, true, fn)
	} else if strings.HasSuffix(me, "run_test_file") {
		tc.RunOneFile(args, fn)
	} else if strings.HasSuffix(me, "run_test_dir") {
		tc.RunOneDir(args, true, fn)
	} else if strings.HasSuffix(me, "deploycode") {
		getDeployedCode()
	} else {
		fmt.Printf("NOT RUN \n")
	}
	os.Exit(0)
}
