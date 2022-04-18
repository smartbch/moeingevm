package testcase

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	coretypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/smartbch/moeingads"
	"github.com/smartbch/moeingads/store/rabbit"

	"github.com/smartbch/moeingevm/types"
	"github.com/smartbch/moeingevm/utils"
)

/*
#include "../host_bridge/bridge.h"
*/
import "C"

//type evmc_address = C.struct_evmc_address
//type evmc_bytes32 = C.struct_evmc_bytes32
//type added_log = C.struct_added_log

//func toArr20(addr *evmc_address) [20]byte {
//	var arr [20]byte
//	for i := range arr {
//		arr[i] = byte(addr.bytes[i])
//	}
//	return arr
//}

//func toArr32(addr *evmc_bytes32) [32]byte {
//	var arr [32]byte
//	for i := 0; i < 32; i++ {
//		arr[i] = byte(addr.bytes[i])
//	}
//	return arr
//}

func isAllZero(bz []byte) bool {
	for _, b := range bz {
		if b != 0 {
			return false
		}
	}
	return true
}

type TestCase struct {
	Name      string
	ImplState WorldState
	RefState  WorldState
	Blocks    []*TestBlock
}

func NewTestCase(s string) TestCase {
	return TestCase{
		Name:      s,
		ImplState: NewWorldState(),
		RefState:  NewWorldState(),
		Blocks:    make([]*TestBlock, 0, 1),
	}
}

type BasicAccount struct {
	Balance  uint256.Int
	Nonce    uint64
	Sequence uint64
}

type BytecodeInfo struct {
	Bytecode []byte
	Codehash [32]byte
}

type EvmLog struct {
	Address [20]byte
	Data    []byte
	Topics  [][32]byte
}

type StorageKey struct {
	AccountSeq uint64
	Key        [32]byte
}

func StorageKeyLess(k1, k2 StorageKey) bool {
	if k1.AccountSeq < k2.AccountSeq {
		return true
	}
	if k1.AccountSeq > k2.AccountSeq {
		return false
	}
	return bytes.Compare(k1.Key[:], k2.Key[:]) < 0
}

type WorldState struct {
	CreationCounters [256]uint64
	BlockHashes      [512][32]byte
	Bytecodes        map[[20]byte]BytecodeInfo
	Accounts         map[[20]byte]*BasicAccount
	Values           map[StorageKey][]byte
}

func NewWorldState() WorldState {
	return WorldState{
		Bytecodes: make(map[[20]byte]BytecodeInfo),
		Accounts:  make(map[[20]byte]*BasicAccount),
		Values:    make(map[StorageKey][]byte),
	}
}

func GetWorldStateFromMads(mads *moeingads.MoeingADS) *WorldState {
	world := NewWorldState()
	mads.ScanAll(func(key, value []byte) {
		if bytes.Equal(key, types.StandbyTxQueueKey[:]) {
			return
		}
		if len(key) != 8 {
			panic(fmt.Sprintf("Strange Key %v", key))
		}
		if 64 <= key[0] && key[0] < 64+128 { // in the range for rabbit
			cv := rabbit.BytesToCachedValue(value)
			UpdateWorldState(&world, cv.GetKey(), cv.GetValue())
		}
	})
	return &world
}

func CompareWorldState(stateA, stateB *WorldState) (bool, error) {
	if !reflect.DeepEqual(stateA.Accounts, stateB.Accounts) {
		return false, errors.New("accounts not equal")
	}
	if !reflect.DeepEqual(stateA.Bytecodes, stateB.Bytecodes) {
		return false, errors.New("bytecode not equal")
	}
	if !reflect.DeepEqual(stateA.Values, stateB.Values) {
		return false, errors.New("value not equal")
	}
	if stateA.BlockHashes != stateB.BlockHashes {
		return false, errors.New("block hash not equal")
	}
	if stateA.CreationCounters != stateB.CreationCounters {
		return false, errors.New("creation counters not equal")
	}
	return true, nil
}

func UpdateWorldState(world *WorldState, key, value []byte) {
	if key[0] == types.CREATION_COUNTER_KEY {
		world.CreationCounters[int(key[0])] = binary.BigEndian.Uint64(value)
	} else if key[0] == types.ACCOUNT_KEY {
		var addr [20]byte
		copy(addr[:], key[1:])
		accInfo := types.NewAccountInfo(value)
		world.Accounts[addr] = &BasicAccount{
			Sequence: accInfo.Sequence(),
			Nonce:    accInfo.Nonce(),
		}
		world.Accounts[addr].Balance.SetBytes32(accInfo.BalanceSlice())
	} else if key[0] == types.BYTECODE_KEY {
		var addr [20]byte
		copy(addr[:], key[1:])
		bi := BytecodeInfo{Bytecode: value[33:]}
		copy(bi.Codehash[:], value[1:33]) // value[0] is version byte
		world.Bytecodes[addr] = bi
	} else if key[0] == types.VALUE_KEY {
		skey := StorageKey{AccountSeq: binary.BigEndian.Uint64(key[1:9])}
		copy(skey.Key[:], key[9:])
		world.Values[skey] = append([]byte{}, value...)
	} else if bytes.Equal([]byte{types.CURR_BLOCK_KEY}, key) {
		//Is OK
	} else {
		fmt.Printf("Why key %v value %v\n", key, value)
		panic("Unknown Key")
	}
}

func (world WorldState) SumAllBalance() *uint256.Int {
	res := uint256.NewInt(0)
	for _, acc := range world.Accounts {
		res.Add(res, &acc.Balance)
	}
	return res
}

func (world WorldState) Clone() (out WorldState) {
	out.CreationCounters = world.CreationCounters
	out.BlockHashes = world.BlockHashes
	out.Bytecodes = make(map[[20]byte]BytecodeInfo, len(world.Bytecodes))
	out.Accounts = make(map[[20]byte]*BasicAccount, len(world.Accounts))
	out.Values = make(map[StorageKey][]byte, len(world.Values))
	for k, v := range world.Bytecodes {
		out.Bytecodes[k] = BytecodeInfo{
			Bytecode: append([]byte{}, v.Bytecode...),
			Codehash: v.Codehash,
		}
	}
	for k, v := range world.Accounts {
		out.Accounts[k] = &BasicAccount{
			Balance:  v.Balance,
			Nonce:    v.Nonce,
			Sequence: v.Sequence,
		}
	}
	for k, v := range world.Values {
		out.Values[k] = append([]byte{}, v...)
	}
	return
}

// ===================================

type Tx struct {
	From     [20]byte
	To       [20]byte
	Nonce    uint64
	Value    uint256.Int
	GasPrice uint256.Int
	Gas      uint64
	Data     []byte
}

func (tx Tx) ToEthTx() *coretypes.Transaction {
	t := coretypes.NewTransaction(tx.Nonce, tx.To, tx.Value.ToBig(), tx.Gas, tx.GasPrice.ToBig(), tx.Data)
	t, _ = t.WithSignature(&DumbSigner{}, tx.From[:])
	return t
}

type DumbSigner struct {
}

var _ coretypes.Signer = (*DumbSigner)(nil)

func (signer *DumbSigner) ChainID() *big.Int {
	return big.NewInt(1)
}

// Sender returns the sender address of the transaction.
func (signer *DumbSigner) Sender(tx *coretypes.Transaction) (addr common.Address, err error) {
	_, r, _ := tx.RawSignatureValues()
	r.FillBytes(addr[:])
	return
}

// SignatureValues returns the raw R, S, V values corresponding to the
// given signature.
func (signer *DumbSigner) SignatureValues(tx *coretypes.Transaction, sig []byte) (r, s, v *big.Int, err error) {
	r, s, v = &big.Int{}, &big.Int{}, &big.Int{}
	r.SetBytes(sig)
	return
}

// Hash returns the hash to be signed.
func (signer *DumbSigner) Hash(tx *coretypes.Transaction) common.Hash {
	return common.Hash{}
}

// Equal returns true if the given signer is the same as the receiver.
func (signer *DumbSigner) Equal(s coretypes.Signer) bool {
	if ss, ok := s.(*DumbSigner); ok {
		return signer == ss
	}
	return false
}

type TestBlock struct {
	types.BlockInfo
	TxList []*Tx
}

func NewTestBlock() *TestBlock {
	return &TestBlock{
		TxList: make([]*Tx, 0, 1),
	}
}

func hexCharToNum(c uint8) uint8 {
	if uint8('0') <= c && c <= uint8('9') {
		return c - uint8('0')
	}
	if uint8('A') <= c && c <= uint8('F') {
		return c - uint8('A') + 10
	}
	if uint8('a') <= c && c <= uint8('f') {
		return c - uint8('a') + 10
	}
	return 0
}

func hexToNum(hi, lo uint8) uint8 {
	return (hexCharToNum(hi) << 4) | hexCharToNum(lo)
}

func fillBytes(tokens []string, data *[]byte) []string {
	if strings.HasPrefix(tokens[0], "0x") {
		var err error
		*data, err = hex.DecodeString(tokens[0][2:])
		if err != nil {
			panic(err)
		}
		return tokens[1:]
	}
	size, _ := strconv.Atoi(tokens[0])
	tokens = tokens[1:]
	*data = make([]byte, 0, size)
	for i := 0; i < size; i++ {
		c, _ := strconv.Atoi(tokens[0])
		tokens = tokens[1:]
		if !(0 <= c && c <= 255) {
			panic("out of range")
		}
		*data = append(*data, uint8(c))
	}
	return tokens
}

func fillSliceFromHex(tokens []string, arr []byte) []string {
	addrStr := tokens[0]
	tokens = tokens[1:]
	if len(addrStr)%2 != 0 {
		panic("invalid length")
	}

	strPos := len(addrStr) - 2
	for i := 0; i < len(arr); i++ {
		if strPos >= 2 { // skip "0x"
			arr[len(arr)-1-i] = hexToNum(addrStr[strPos], addrStr[strPos+1])
		} else {
			arr[len(arr)-1-i] = 0
		}
		strPos -= 2
	}
	return tokens
}

func fillKV(tokens []string, k []byte, v *[]byte) []string {
	tokens = fillSliceFromHex(tokens, k)
	vStr := tokens[0]
	tokens = tokens[1:]
	if len(vStr)%2 != 0 {
		panic("invalid length")
	}
	for strPos := 2; /*skip 0x*/ strPos < len(vStr); strPos += 2 {
		*v = append(*v, hexToNum(vStr[strPos], vStr[strPos+1]))
	}
	return tokens
}

func fillUInt256(tokens []string, v *uint256.Int) []string {
	s := tokens[0]
	tokens = tokens[1:]
	bigInt := &big.Int{}
	_, ok := bigInt.SetString(s, 10)
	if !ok {
		panic("Invalid uint256")
	}
	u, overflow := uint256.FromBig(bigInt)
	if overflow {
		panic("overflow uint256")
	}
	v.Set(u)
	return tokens
}

func createAccount(currState *WorldState, addr [20]byte, b *uint256.Int) {
	if _, ok := currState.Accounts[addr]; !ok {
		ba := &BasicAccount{Sequence: ^uint64(0)}
		ba.Balance.Set(b)
		currState.Accounts[addr] = ba
	}
}

func checkTestCases(filename string, caseList []TestCase) {
	fout, _ := os.Create("./cases.dump")
	PrintTestCases(fout, caseList)
	fout.Close()
	cmd := exec.Command("diff", filename, "./cases.dump")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Compare Error %s\n", filename)
		panic(err)
	} else {
		fmt.Printf("File Check OK %s\n", filename)
	}
}

func checkSequence(currAddress [20]byte, seqPtr *uint64, currState *WorldState) {
	if currState.Accounts[currAddress].Sequence == ^uint64(0) {
		currSeq := currState.CreationCounters[currAddress[0]]
		currState.CreationCounters[currAddress[0]]++
		currSeq = (currSeq << 8) | uint64(currAddress[0])
		currState.Accounts[currAddress].Sequence = currSeq
		*seqPtr = currSeq
	}
}

func atoi(s string) uint64 {
	n, _ := strconv.Atoi(s)
	return uint64(n)
}

func ReadTestCasesFromString(str string) (result []TestCase) {
	return readTestCases(strings.NewReader(str))
}

func ReadTestCases(filename string) (result []TestCase) {
	infile, _ := os.Open(filename)
	defer infile.Close()
	result = readTestCases(infile)
	checkTestCases(filename, result)
	return
}

func readTestCases(infile io.Reader) (result []TestCase) {
	var currCase *TestCase
	var currState *WorldState
	var currBlock *TestBlock
	var currTx *Tx
	var currAddress [20]byte
	currSeq := ^uint64(0)
	scanner := bufio.NewScanner(infile)
	scanner.Buffer(nil, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		//fmt.Printf("Reading------- %s\n", line)
		tokens := strings.Fields(line)
		cmd, tokens := tokens[0], tokens[1:]
		switch {
		case cmd == "test":
			result = append(result, NewTestCase(tokens[0]))
			currCase = &result[len(result)-1]
		case cmd == "pre":
			currState = &currCase.ImplState
		case cmd == "post":
			currState = &currCase.RefState
			//fmt.Printf("@@post !\n")
		case cmd == "addr":
			fillSliceFromHex(tokens, currAddress[:])
			createAccount(currState, currAddress, uint256.NewInt(0).SetUint64(0))
			//fmt.Printf("@@createAccount  %v\n", currAddress)
		case cmd == "acc_nonce":
			currState.Accounts[currAddress].Nonce = atoi(tokens[0])
		case cmd == "balance":
			fillUInt256(tokens, &currState.Accounts[currAddress].Balance)
		case cmd == "code":
			var bz []byte
			fillBytes(tokens, &bz)
			if len(bz) > 0 {
				checkSequence(currAddress, &currSeq, currState)
				currState.Bytecodes[currAddress] = BytecodeInfo{
					Bytecode: bz,
					Codehash: crypto.Keccak256Hash(bz),
				}
			}
		case cmd == "kv":
			var k [32]byte
			var v []byte
			fillKV(tokens, k[:], &v)
			if !isAllZero(v) {
				checkSequence(currAddress, &currSeq, currState)
				skey := StorageKey{AccountSeq: currSeq, Key: k}
				currState.Values[skey] = v
			}
		case cmd == "blocks":
			/* do nothing*/
		case cmd == "block":
			currCase.Blocks = append(currCase.Blocks, NewTestBlock())
			currBlock = currCase.Blocks[len(currCase.Blocks)-1]
			currBlock.ChainId = uint256.NewInt(0).SetUint64(1).Bytes32()
		case cmd == "coinbase":
			fillSliceFromHex(tokens, currBlock.Coinbase[:])
		case cmd == "height":
			currBlock.Number = int64(atoi(tokens[0]))
		case cmd == "difficulty":
			difficulty := uint256.NewInt(0)
			fillUInt256(tokens, difficulty)
			currBlock.Difficulty = difficulty.Bytes32()
		case cmd == "gaslimit":
			currBlock.GasLimit = int64(atoi(tokens[0]))
		case cmd == "timestamp":
			currBlock.Timestamp = int64(atoi(tokens[0]))
		case cmd == "tx":
			currBlock.TxList = append(currBlock.TxList, &Tx{})
			currTx = currBlock.TxList[len(currBlock.TxList)-1]
		case cmd == "from":
			fillSliceFromHex(tokens, currTx.From[:])
		case cmd == "to":
			fillSliceFromHex(tokens, currTx.To[:])
		case cmd == "nonce":
			currTx.Nonce = atoi(tokens[0])
		case cmd == "value":
			fillUInt256(tokens, &currTx.Value)
		case cmd == "gasprice":
			fillUInt256(tokens, &currTx.GasPrice)
		case cmd == "gas":
			currTx.Gas = atoi(tokens[0])
		case cmd == "data":
			fillBytes(tokens, &currTx.Data)
		default:
			panic("Unknown cmd " + cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}
	return
}

// ==============================================

func toHex(bz []byte) string {
	return hex.EncodeToString(bz)
}

var (
	// record pending gas fee and refund
	systemContractAddress = [20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		byte('s'), byte('y'), byte('s'), byte('t'), byte('e'), byte('m')}
)

func PrintWorldState(fout io.Writer, state *WorldState) {
	addrList := make([][20]byte, 0, len(state.Accounts))
	for addr := range state.Accounts {
		addrList = append(addrList, addr)
	}
	sort.Slice(addrList, func(i, j int) bool {
		return bytes.Compare(addrList[i][:], addrList[j][:]) < 0
	})
	for _, addr := range addrList {
		if addr == systemContractAddress {
			continue
		}
		acc := state.Accounts[addr]
		fmt.Fprintf(fout, " addr 0x%s\n", toHex(addr[:]))
		fmt.Fprintf(fout, "  acc_nonce %d\n", acc.Nonce)
		fmt.Fprintf(fout, "  balance %s\n", acc.Balance.ToBig().String())
		if _, ok := state.Bytecodes[addr]; !ok {
			fmt.Fprintf(fout, "  code 0\n")
		} else {
			code := state.Bytecodes[addr]
			fmt.Fprintf(fout, "  code %d", len(code.Bytecode))
			for _, d := range code.Bytecode {
				fmt.Fprintf(fout, " %d", int(d))
			}
			fmt.Fprintf(fout, "\n")
		}
		skeyList := make([]StorageKey, 0, len(state.Values))
		for skey := range state.Values {
			skeyList = append(skeyList, skey)
		}
		sort.Slice(skeyList, func(i, j int) bool {
			return StorageKeyLess(skeyList[i], skeyList[j])
		})
		for _, skey := range skeyList {
			v := state.Values[skey]
			if skey.AccountSeq != acc.Sequence {
				continue
			}
			fmt.Fprintf(fout, "   kv 0x%s 0x%s\n", toHex(skey.Key[:]), toHex(v[:]))
		}
	}
}

func PrintBlock(fout io.Writer, block *TestBlock) {
	fmt.Fprintf(fout, "  coinbase 0x%s\n", toHex(block.Coinbase[:]))
	fmt.Fprintf(fout, "  height %d\n", block.Number)
	fmt.Fprintf(fout, "  difficulty %s\n", utils.BigIntFromSlice32(block.Difficulty[:]).String())
	fmt.Fprintf(fout, "  gaslimit %d\n", block.GasLimit)
	fmt.Fprintf(fout, "  timestamp %d\n", block.Timestamp)
	idx := 0
	for _, tx := range block.TxList {
		fmt.Fprintf(fout, "  tx %d\n", idx)
		idx++
		//fmt.Printf("tx %v\n", tx)
		fmt.Fprintf(fout, "   from 0x%s\n", toHex(tx.From[:]))
		fmt.Fprintf(fout, "   nonce %d\n", tx.Nonce)
		fmt.Fprintf(fout, "   to 0x%s\n", toHex(tx.To[:]))
		fmt.Fprintf(fout, "   value %s\n", tx.Value.ToBig().String())
		fmt.Fprintf(fout, "   gasprice %s\n", tx.GasPrice.ToBig().String())
		fmt.Fprintf(fout, "   gas %d\n", tx.Gas)
		fmt.Fprintf(fout, "   data %d", len(tx.Data))
		for _, d := range tx.Data {
			fmt.Fprintf(fout, " %d", int(d))
		}
		fmt.Fprintf(fout, "\n")
	}
}

func PrintTestCases(fout io.Writer, cases []TestCase) {
	for _, currCase := range cases {
		fmt.Fprintf(fout, "test %s\n", currCase.Name)
		fmt.Fprintf(fout, "pre\n")
		PrintWorldState(fout, &currCase.ImplState)
		fmt.Fprintf(fout, "post\n")
		PrintWorldState(fout, &currCase.RefState)
		fmt.Fprintf(fout, "blocks\n")
		for i, block := range currCase.Blocks {
			fmt.Fprintf(fout, " block %d\n", i)
			PrintBlock(fout, block)
		}
	}
}

// =================================

type RunTestCaseFn func(filename string, theCase *TestCase, printLog bool)

func RunOneCase(args []string, printLog bool, runTestCase RunTestCaseFn) {
	if len(args) != 3 {
		fmt.Printf("Usage %s test_file test_name\n", args[0])
		return
	}

	filename := args[1]
	cases := ReadTestCases(filename)
	testname := args[2]
	foundIt := false
	for i := range cases {
		theCase := &cases[i]
		if theCase.Name == testname {
			foundIt = true
			runTestCase(filename, theCase, printLog)
			break
		}
	}
	if !foundIt {
		fmt.Printf("No such test case: %s\n", testname)
	}
}

func RunOneFile(args []string, runTestCase RunTestCaseFn) {
	if len(args) != 2 {
		fmt.Printf("Usage %s test_file\n", args[0])
		return
	}

	filename := args[1]
	cases := ReadTestCases(filename)
	for i := range cases {
		theCase := &cases[i]
		runTestCase(filename, theCase, false)
	}
}

func RunOneDir(args []string, printLog bool, runTestCase RunTestCaseFn) {
	if len(args) != 2 {
		fmt.Printf("Usage: %s dir_name\n", args[0])
		return
	}
	dirname := args[1]
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		filename := file.Name()
		if !strings.HasSuffix(filename, ".txt") {
			continue
		}
		filename = dirname + "/" + filename
		cases := ReadTestCases(filename)
		if cases == nil {
			panic("no case")
		}
		for i := range cases {
			theCase := &cases[i]
			runTestCase(filename, theCase, printLog)
		}
	}
}

func AddBlockReward(currState *WorldState, addr [20]byte, b *uint256.Int) {
	if acc, ok := currState.Accounts[addr]; ok {
		acc.Balance.Set((&uint256.Int{}).Add(&acc.Balance, b))
		currState.Accounts[addr] = acc
	} else {
		ba := &BasicAccount{Sequence: ^uint64(0)}
		ba.Balance.Set(b)
		currState.Accounts[addr] = ba

	}
}
