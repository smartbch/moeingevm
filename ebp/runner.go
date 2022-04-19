package ebp

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/smartbch/moeingevm/types"
	"github.com/smartbch/moeingevm/utils"
)

//#include "../evmwrap/host_bridge/bridge.h"
//int64_t zero_depth_call_wrap(evmc_bytes32 gas_price,
//                             int64_t gas_limit,
//                             const evmc_address* destination,
//                             const evmc_address* sender,
//                             const evmc_bytes32* value,
//                             const uint8_t* input_data,
//                             size_t input_size,
//                             const struct block_info* block,
//                             int collector_handler,
//                             bool need_gas_estimation,
//                             enum evmc_revision revision,
//                             bridge_query_executor_fn query_executor_fn);
import "C"

type (
	//evmc_message             = C.struct_evmc_message
	evmc_address             = C.struct_evmc_address
	evmc_bytes32             = C.struct_evmc_bytes32
	evmc_result              = C.struct_evmc_result
	internal_tx_call         = C.struct_internal_tx_call
	internal_tx_return       = C.struct_internal_tx_return
	changed_account          = C.struct_changed_account
	changed_creation_counter = C.struct_changed_creation_counter
	changed_bytecode         = C.struct_changed_bytecode
	changed_value            = C.struct_changed_value
	added_log                = C.struct_added_log
	all_changed              = C.struct_all_changed
	block_info               = C.struct_block_info
	big_buffer               = C.struct_big_buffer
	small_buffer             = C.struct_small_buffer
)

const (
	EnableRWList = false
)

var PredefinedContractManager map[common.Address]types.SystemContractExecutor

func RegisterPredefinedContract(ctx *types.Context, address common.Address, executor types.SystemContractExecutor) {
	PredefinedContractManager[address] = executor
	if !executor.IsSystemContract(address) {
		panic(fmt.Sprintf("contract %s is not system contract", address.String()))
	}
	executor.Init(ctx)
}

func init() {
	PredefinedContractManager = make(map[common.Address]types.SystemContractExecutor)
}

// This is a global variable. The parameter 'collector_handler' passed to zero_depth_call_wrap is
// an index to select one TxRunner from this global variable.
var Runners []*TxRunner

const (
	RpcRunnersIdStart int = 10000
	RpcRunnersCount   int = 256
	SMALL_BUF_SIZE    int = int(C.SMALL_BUF_SIZE)
)

var AdjustGasUsed = true // It's a global variable because in tests we must change it to false to be compatible

// Its usage is similar with Runners. Runners are for transactions in block. RpcRunners are for transactions
// in Web3 RPC: call and estimateGas.
var RpcRunners [RpcRunnersCount]*TxRunner

var RpcRunnerLocks [RpcRunnersCount]spinLock

type spinLock uint32

func (sl *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

// try to abtain a lock and return false when failed
func (sl *spinLock) TryLock() bool {
	return atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1)
}

func getFreeRpcRunnerAndLockIt() int {
	for {
		for i := range RpcRunnerLocks {
			if RpcRunnerLocks[i].TryLock() {
				return i
			}
		}
		runtime.Gosched()
	}
}

func getRunner(i int) (runner *TxRunner) {
	if i < RpcRunnersIdStart {
		runner = Runners[i]
		runner.ForRpc = false
	} else {
		runner = RpcRunners[i-RpcRunnersIdStart]
		runner.ForRpc = true
	}
	return
}

type TxRunner struct {
	Ctx       *types.Context
	GasUsed   uint64
	FeeRefund uint256.Int
	Tx        *types.TxToRun
	Logs      []types.EvmLog
	Status    int
	OutData   []byte
	ForRpc    bool

	CreatedContractAddress common.Address

	InternalTxCalls   []types.InternalTxCall
	InternalTxReturns []types.InternalTxReturn

	RwLists *types.ReadWriteLists
}

func NewTxRunner(ctx *types.Context, tx *types.TxToRun) *TxRunner {
	return &TxRunner{
		Ctx:     ctx,
		Tx:      tx,
		RwLists: &types.ReadWriteLists{},
	}
}

func toAddress(addr *evmc_address) (arr common.Address) {
	for i := range arr {
		arr[i] = byte(addr.bytes[i])
	}
	return
}

func toHash(bytes32 *evmc_bytes32) (hash common.Hash) {
	for i := range hash {
		hash[i] = byte(bytes32.bytes[i])
	}
	return
}

func writeSliceWithCBytes32(bz []byte, b32 *evmc_bytes32) {
	for i := 0; i < 32; i++ {
		bz[i] = uint8(b32.bytes[i])
	}
}

func writeCBytes32WithSlice(b32 *evmc_bytes32, bz []byte) {
	for i := 0; i < 32; i++ {
		b32.bytes[i] = C.uint8_t(bz[i])
	}
}

func writeCBytes20WithArray(arr *evmc_address, bz [20]byte) {
	for i := 0; i < 20; i++ {
		arr.bytes[i] = C.uint8_t(bz[i])
	}
}

//Following are some getter/setter functions which provide world state to the C environment and
//apply the changes made by the C environment to world state.

func (runner *TxRunner) getCreationCounter(lsb uint8) uint64 {
	k := types.GetCreationCounterKey(lsb)
	v := runner.Ctx.Rbt.Get(k)
	if v == nil {
		return 0
	}
	counter := binary.BigEndian.Uint64(v)
	if EnableRWList {
		runner.RwLists.CreationCounterRList = append(runner.RwLists.CreationCounterRList,
			types.CreationCounterRWOp{Lsb: lsb, Counter: counter})
	}
	return counter
}

func (runner *TxRunner) changeCreationCounter(chg_counter *changed_creation_counter) {
	k := types.GetCreationCounterKey(uint8(chg_counter.lsb))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(chg_counter.counter))
	runner.Ctx.Rbt.Set(k, buf[:])
	if !EnableRWList {
		return
	}
	runner.RwLists.CreationCounterWList = append(runner.RwLists.CreationCounterWList,
		types.CreationCounterRWOp{Lsb: uint8(chg_counter.lsb), Counter: uint64(chg_counter.counter)})
}

func (runner *TxRunner) getAccountInfo(addr_ptr *evmc_address, balance *evmc_bytes32, nonce *C.uint64_t, sequence *C.uint64_t) {
	addr := toAddress(addr_ptr)
	acc := runner.Ctx.GetAccount(addr)
	if acc == nil {
		*nonce = ^C.uint64_t(0) // nonce with all ones means a non-existant account
		return
	}
	writeCBytes32WithSlice(balance, acc.BalanceSlice())
	*nonce = C.uint64_t(binary.BigEndian.Uint64(acc.NonceSlice()))
	*sequence = C.uint64_t(binary.BigEndian.Uint64(acc.SequenceSlice()))
	if !EnableRWList {
		return
	}
	op := types.AccountRWOp{Account: acc.Bytes(), Addr: addr}
	runner.RwLists.AccountRList = append(runner.RwLists.AccountRList, op)
}

func (runner *TxRunner) changeAccount(chg_acc *changed_account) {
	addr := toAddress(chg_acc.address)
	k := types.GetAccountKey(addr)
	acc := &types.AccountInfo{}
	if chg_acc.delete_me {
		runner.Ctx.Rbt.Delete(k)
	} else {
		acc = types.ZeroAccountInfo()
		binary.BigEndian.PutUint64(acc.NonceSlice(), uint64(chg_acc.nonce))
		binary.BigEndian.PutUint64(acc.SequenceSlice(), uint64(chg_acc.sequence))
		writeSliceWithCBytes32(acc.BalanceSlice(), &chg_acc.balance)
		runner.Ctx.Rbt.Set(k, acc.Bytes())
	}
	if !EnableRWList {
		return
	}
	if addr == runner.Tx.From {
		return // We cannot get correct data here without refund
	}
	op := types.AccountRWOp{Account: acc.Bytes(), Addr: addr}
	runner.RwLists.AccountWList = append(runner.RwLists.AccountWList, op)
}

func (runner *TxRunner) getBytecode(addr_ptr *evmc_address, codehash_ptr *evmc_bytes32, buf *big_buffer, size *C.size_t) {
	addr := toAddress(addr_ptr)
	bi := runner.Ctx.GetCode(addr)
	if bi == nil {
		*size = C.size_t(0)
		return
	}
	bs := bi.BytecodeSlice()
	*size = C.size_t(len(bs))
	for i := range bs {
		buf.data[i] = C.uint8_t(bs[i])
	}
	writeCBytes32WithSlice(codehash_ptr, bi.CodeHashSlice())
	if !EnableRWList {
		return
	}
	op := types.BytecodeRWOp{Bytecode: bi.Bytes(), Addr: addr}
	runner.RwLists.BytecodeRList = append(runner.RwLists.BytecodeRList, op)
}

func (runner *TxRunner) changeBytecode(chg_bytecode *changed_bytecode) {
	addr := toAddress(chg_bytecode.address)
	k := types.GetBytecodeKey(addr)
	var bz []byte
	if chg_bytecode.bytecode_size == 0 {
		runner.Ctx.Rbt.Delete(k)
	} else {
		bz = make([]byte, 33, 33+chg_bytecode.bytecode_size)
		bz[0] = 0 // version byte is zero
		writeSliceWithCBytes32(bz[1:33], chg_bytecode.codehash)
		bz = append(bz, C.GoStringN(chg_bytecode.bytecode_data, chg_bytecode.bytecode_size)...)
		runner.Ctx.Rbt.Set(k, bz)
	}
	if !EnableRWList {
		return
	}
	op := types.BytecodeRWOp{Bytecode: bz, Addr: addr}
	runner.RwLists.BytecodeWList = append(runner.RwLists.BytecodeWList, op)
}

func (runner *TxRunner) getValue(acc_seq C.uint64_t, key_ptr *C.char, buf *big_buffer, size *C.size_t) {
	seq := uint64(acc_seq)
	key := C.GoStringN(key_ptr, 32)
	bs := runner.Ctx.GetStorageAt(seq, key)
	*size = C.size_t(len(bs))
	for i := range bs {
		buf.data[i] = C.uint8_t(bs[i])
	}
	if !EnableRWList {
		return
	}
	op := types.StorageRWOp{Seq: seq, Key: key, Value: bs}
	runner.RwLists.StorageRList = append(runner.RwLists.StorageRList, op)
}

func (runner *TxRunner) changeValue(chg_value *changed_value) {
	seq := uint64(chg_value.account_seq)
	key := C.GoStringN(chg_value.key_ptr, 32)
	k := types.GetValueKey(seq, key)
	var bz []byte
	if chg_value.value_size == 0 {
		runner.Ctx.Rbt.Delete(k)
	} else {
		bz = C.GoBytes(unsafe.Pointer(chg_value.value_data), chg_value.value_size)
		runner.Ctx.Rbt.Set(k, bz)
	}
	if !EnableRWList {
		return
	}
	op := types.StorageRWOp{Seq: seq, Key: key, Value: bz}
	runner.RwLists.StorageWList = append(runner.RwLists.StorageWList, op)
}

//hash => height; height => block in db
func (runner *TxRunner) getBlockHash(num C.uint64_t) (result evmc_bytes32) {
	hash := runner.Ctx.GetBlockHashByHeight(uint64(num))
	writeCBytes32WithSlice(&result, hash[:])
	if !EnableRWList {
		return
	}
	op := types.BlockHashOp{Height: uint64(num), Hash: hash}
	runner.RwLists.BlockHashList = append(runner.RwLists.BlockHashList, op)
	return
}

// Refund gas fee to the sender according to the real consumed gas
func (runner *TxRunner) refundGasFee(ret_value *evmc_result, refund C.uint64_t) {
	if runner.ForRpc {
		return
	}
	gasUsed := runner.Tx.Gas - uint64(ret_value.gas_left)
	if AdjustGasUsed {
		if gasUsed*4 < runner.Tx.Gas {
			gasUsed = runner.Tx.Gas
		} else if gasUsed*2 < runner.Tx.Gas {
			gasUsed = (runner.Tx.Gas + gasUsed) / 2
		}
	}
	half := (gasUsed + 1) / 2
	if gasUsed < uint64(refund)+half { // can refund no more than half
		gasUsed = half
	} else {
		gasUsed = gasUsed - uint64(refund)
	}

	k := types.GetAccountKey(runner.Tx.From)

	var returnedGasFee uint256.Int
	gasPrice := utils.U256FromSlice32(runner.Tx.GasPrice[:])
	returnedGasFee.Mul(uint256.NewInt(0).SetUint64(runner.Tx.Gas-gasUsed), gasPrice)
	acc := types.NewAccountInfo(runner.Ctx.Rbt.Get(k))
	x := utils.U256FromSlice32(acc.BalanceSlice())
	x.Add(x, &returnedGasFee)
	copy(acc.BalanceSlice(), utils.U256ToSlice32(x))
	runner.Ctx.Rbt.Set(k, acc.Bytes())
	runner.FeeRefund = returnedGasFee
	runner.GasUsed = gasUsed
	if !EnableRWList {
		return
	}
	op := types.AccountRWOp{Account: acc.Bytes(), Addr: runner.Tx.From}
	runner.RwLists.AccountWList = append(runner.RwLists.AccountWList, op)
}

func (runner *TxRunner) GetGasFee() *uint256.Int {
	return uint256.NewInt(0).Mul(uint256.NewInt(runner.GasUsed),
		uint256.NewInt(0).SetBytes(runner.Tx.GasPrice[:]))
}

func convertLog(log *added_log) (res types.EvmLog) {
	if log.topic1 != nil {
		res.Topics = append(res.Topics, toHash(log.topic1))
	}
	if log.topic2 != nil {
		res.Topics = append(res.Topics, toHash(log.topic2))
	}
	if log.topic3 != nil {
		res.Topics = append(res.Topics, toHash(log.topic3))
	}
	if log.topic4 != nil {
		res.Topics = append(res.Topics, toHash(log.topic4))
	}
	res.Address = toAddress(log.contract_addr)
	res.Data = C.GoBytes(unsafe.Pointer(log.data), log.size)
	return
}

func convertTxCalls(data_ptr C.size_t, msg *internal_tx_call) (txCall types.InternalTxCall) {
	txCall.Kind = int(msg.kind)
	txCall.Flags = uint32(msg.flags)
	txCall.Depth = int32(msg.depth)
	txCall.Gas = int64(msg.gas)
	txCall.Destination = toAddress(&msg.destination)
	txCall.Sender = toAddress(&msg.sender)
	txCall.Input = C.GoBytes(unsafe.Pointer(uintptr(data_ptr+msg.input_offset)), C.int(msg.input_size))
	txCall.Value = toHash(&msg.value)
	return
}

func convertTxReturns(data_ptr C.size_t, ret *internal_tx_return) (txReturn types.InternalTxReturn) {
	txReturn.StatusCode = int(ret.status_code)
	txReturn.GasLeft = int64(ret.gas_left)
	txReturn.Output = C.GoBytes(unsafe.Pointer(uintptr(data_ptr+ret.output_offset)), C.int(ret.output_size))
	txReturn.CreateAddress = toAddress(&ret.create_address)
	return
}

// This function will be called by the C environment to feed changes to Go environment, before the
// C environment cleans up and exits.
func (runner *TxRunner) collectResult(result *all_changed, ret_value *evmc_result) {
	if result == nil {
		runner.Status = int(ret_value.status_code)
		runner.refundGasFee(ret_value, 0)
		return
	}
	runner.OutData = C.GoBytes(unsafe.Pointer(ret_value.output_data), C.int(ret_value.output_size))
	size := int(result.account_num)
	if size != 0 {
		accounts := (*[1 << 30]changed_account)(unsafe.Pointer(result.accounts))[:size:size]
		for _, elem := range accounts {
			runner.changeAccount(&elem)
		}
	}
	size = int(result.creation_counter_num)
	if size != 0 {
		creation_counters := (*[1 << 30]changed_creation_counter)(unsafe.Pointer(result.creation_counters))[:size:size]
		for _, elem := range creation_counters {
			runner.changeCreationCounter(&elem)
		}
	}
	size = int(result.bytecode_num)
	if size != 0 {
		bytecodes := (*[1 << 30]changed_bytecode)(unsafe.Pointer(result.bytecodes))[:size:size]
		for _, elem := range bytecodes {
			runner.changeBytecode(&elem)
		}
	}
	size = int(result.value_num)
	if size != 0 {
		values := (*[1 << 30]changed_value)(unsafe.Pointer(result.values))[:size:size]
		for _, elem := range values {
			runner.changeValue(&elem)
		}
	}
	size = int(result.log_num)
	if size != 0 {
		logs := (*[1 << 30]added_log)(unsafe.Pointer(result.logs))[:size:size]
		for _, elem := range logs {
			runner.Logs = append(runner.Logs, convertLog(&elem))
		}
	}
	size = int(result.internal_tx_call_num)
	if size != 0 {
		calls := (*[1 << 30]internal_tx_call)(unsafe.Pointer(result.internal_tx_calls))[:size:size]
		for _, elem := range calls {
			runner.InternalTxCalls = append(runner.InternalTxCalls, convertTxCalls(result.data_ptr, &elem))
		}
	}
	size = int(result.internal_tx_return_num)
	if size != 0 {
		returns := (*[1 << 30]internal_tx_return)(unsafe.Pointer(result.internal_tx_returns))[:size:size]
		for _, elem := range returns {
			runner.InternalTxReturns = append(runner.InternalTxReturns, convertTxReturns(result.data_ptr, &elem))
		}
	}
	runner.Status = int(ret_value.status_code)
	runner.refundGasFee(ret_value, result.refund)
	runner.CreatedContractAddress = toAddress(&ret_value.create_address)
}

// Functions below wrap the member functions of TxRunner with pure C function signatures.

//export collect_result
func collect_result(handler C.int, result *all_changed, ret_value *evmc_result) {
	getRunner(int(handler)).collectResult(result, ret_value)
}

//export get_creation_counter
func get_creation_counter(handler C.int, n C.uint8_t) C.uint64_t {
	return C.uint64_t(getRunner(int(handler)).getCreationCounter(uint8(n)))
}

//export get_account_info
func get_account_info(handler C.int, addr *evmc_address, balance *evmc_bytes32, nonce *C.uint64_t, sequence *C.uint64_t) {
	getRunner(int(handler)).getAccountInfo(addr, balance, nonce, sequence)
}

//export get_bytecode
func get_bytecode(handler C.int, addr *evmc_address, codehash_ptr *evmc_bytes32, buf *big_buffer, size *C.size_t) {
	getRunner(int(handler)).getBytecode(addr, codehash_ptr, buf, size)
}

//export get_value
func get_value(handler C.int, acc_seq C.uint64_t, key_ptr *C.char, buf *big_buffer, size *C.size_t) {
	getRunner(int(handler)).getValue(acc_seq, key_ptr, buf, size)
}

//export get_block_hash
func get_block_hash(handler C.int, num C.uint64_t) (result evmc_bytes32) {
	return getRunner(int(handler)).getBlockHash(num)
}

func runTx(idx int, currBlock *types.BlockInfo) {
	runTxHelper(idx, currBlock, false)
}

func RunTxForRpc(currBlock *types.BlockInfo, estimateGas bool, runner *TxRunner) int64 {
	//fmt.Printf("RunTxForRpc height %d\n", currBlock.Number)
	idx := getFreeRpcRunnerAndLockIt()
	RpcRunners[idx] = runner
	defer func() {
		RpcRunners[idx] = nil
		RpcRunnerLocks[idx].Unlock()
	}()
	return runTxHelper(idx+RpcRunnersIdStart, currBlock, estimateGas)
}

//Start the idx-th TxRunner to run the transaction assigned to it beforehand.
//In this function Go data structures are converted to C data structures and finally
//call the C entrance function 'zero_depth_call_wrap'.
func runTxHelper(idx int, currBlock *types.BlockInfo, estimateGas bool) int64 {
	runner := getRunner(idx)
	if !runner.ForRpc && runner.Tx.Height+types.TOO_OLD_THRESHOLD < uint64(currBlock.Number) {
		runner.Status = types.IGNORE_TOO_OLD_TX
		return 0
	}
	acc, err := runner.Ctx.CheckNonce(runner.Tx.From, runner.Tx.Nonce)
	if !runner.ForRpc && err != nil { // For RPC, we do not care about sender and its nonce
		if err == types.ErrAccountNotExist {
			runner.Status = types.ACCOUNT_NOT_EXIST
		} else if err == types.ErrNonceTooLarge {
			runner.Status = types.TX_NONCE_TOO_LARGE
		} else if err == types.ErrNonceTooSmall {
			runner.Status = types.TX_NONCE_TOO_SMALL
		} else {
			panic("Unknown Error")
		}
		return 0
	}
	if acc != nil {
		// GasFee was deducted in Prepare(), so here we just increase the nonce
		acc.UpdateNonce(acc.Nonce() + 1)
		runner.Ctx.SetAccount(runner.Tx.From, acc)
	}
	var value, gas_price evmc_bytes32
	var to, from evmc_address
	writeCBytes32WithSlice(&value, runner.Tx.Value[:])
	writeCBytes32WithSlice(&gas_price, runner.Tx.GasPrice[:])
	writeCBytes20WithArray(&to, runner.Tx.To)
	writeCBytes20WithArray(&from, runner.Tx.From)

	var bi block_info
	writeCBytes20WithArray(&bi.coinbase, currBlock.Coinbase)
	bi.number = C.int64_t(currBlock.Number)
	bi.timestamp = C.int64_t(currBlock.Timestamp)
	bi.gas_limit = C.int64_t(currBlock.GasLimit)
	bi.cfg.after_xhedge_fork = C.bool(runner.Ctx.IsXHedgeFork())
	writeCBytes32WithSlice(&bi.difficulty, currBlock.Difficulty[:])
	writeCBytes32WithSlice(&bi.chain_id, currBlock.ChainId[:])
	data_ptr := (*C.uint8_t)(nil)
	if len(runner.Tx.Data) != 0 {
		data_ptr = (*C.uint8_t)(unsafe.Pointer(&runner.Tx.Data[0]))
	}
	if executor, exist := PredefinedContractManager[runner.Tx.To]; exist {
		status, logs, gasUsed, out := executor.Execute(runner.Ctx, currBlock, runner.Tx)
		runner.Status = status
		runner.Logs = logs
		runner.GasUsed = gasUsed
		runner.OutData = out
		return int64(gasUsed)
	}

	gasEstimated := C.zero_depth_call_wrap(gas_price,
		C.int64_t(runner.Tx.Gas),
		&to,
		&from,
		&value,
		data_ptr,
		C.size_t(len(runner.Tx.Data)),
		&bi,
		C.int(idx),
		C.bool(estimateGas),
		C.EVMC_ISTANBUL,
		QueryExecutorFn)
	return int64(gasEstimated)
}

func StatusIsFailure(status int) bool {
	return status != int(C.EVMC_SUCCESS)
}

func StatusToStr(status int) string {
	switch status {
	case int(C.EVMC_SUCCESS):
		return "success"
	case int(C.EVMC_FAILURE):
		return "failure"
	case int(C.EVMC_REVERT):
		return "revert"
	case int(C.EVMC_OUT_OF_GAS):
		return "out-of-gas"
	case int(C.EVMC_INVALID_INSTRUCTION):
		return "invalid-instruction"
	case int(C.EVMC_UNDEFINED_INSTRUCTION):
		return "undefined-instruction"
	case int(C.EVMC_STACK_OVERFLOW):
		return "stack-overflow"
	case int(C.EVMC_STACK_UNDERFLOW):
		return "stack-underflow"
	case int(C.EVMC_BAD_JUMP_DESTINATION):
		return "bad-jump-destination"
	case int(C.EVMC_INVALID_MEMORY_ACCESS):
		return "invalid-memory-access"
	case int(C.EVMC_CALL_DEPTH_EXCEEDED):
		return "call-depth-exceeded"
	case int(C.EVMC_STATIC_MODE_VIOLATION):
		return "static-mode-violation"
	case int(C.EVMC_PRECOMPILE_FAILURE):
		return "precompile-failure"
	case int(C.EVMC_CONTRACT_VALIDATION_FAILURE):
		return "contract-validation-failure"
	case int(C.EVMC_ARGUMENT_OUT_OF_RANGE):
		return "argument-out-of-range"
	case int(C.EVMC_INSUFFICIENT_BALANCE):
		return "insufficient-balance"
	case int(C.EVMC_INTERNAL_ERROR):
		return "internal-error"
	case int(C.EVMC_REJECTED):
		return "rejected"
	case int(C.EVMC_OUT_OF_MEMORY):
		return "out-of-memory"
	case types.IGNORE_TOO_OLD_TX:
		return "too-old-and-ignored"
	case types.ACCOUNT_NOT_EXIST:
		return "account-not-exist"
	case types.TX_NONCE_TOO_SMALL:
		return "nonce-too-small"
	}
	return "unknown"
}
