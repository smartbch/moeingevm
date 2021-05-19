package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/tests"
)

// findLine returns the line number for the given offset into data.
func findLine(data []byte, offset int64) (line int) {
	line = 1
	for i, r := range string(data) {
		if int64(i) >= offset {
			return
		}
		if r == '\n' {
			line++
		}
	}
	return
}

func readJSON(reader io.Reader, value interface{}) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading JSON file: %v", err)
	}
	if err = json.Unmarshal(data, &value); err != nil {
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			line := findLine(data, syntaxErr.Offset)
			return fmt.Errorf("JSON syntax error at line %v: %v", line, err)
		}
		return err
	}
	return nil
}

/*
func runStateTestCase0(testName string, testCase *BlockTest) error {
	app := app.NewTestApp()
	defer app.Close()

	vmctrl.EVMVersion = testCase.Json.Network
	config := tests.Forks[testCase.Json.Network]
	for _, block := range testCase.Json.Blocks {
		header := makeHeader(config, &block)
		ctx := app.NewContext(false, header)
		ctx = ctx.WithBlockGasMeter(sdk.NewGasMeter(block.Header.GasLimit))
		cacheCtx, _ := ctx.CacheContext()
		//makeCoinBaseAcc(cacheCtx, app, header.ProposerAddress)
		err := makePreState(cacheCtx, app, testCase)
		if err != nil {
			return err
		}
		msgs, intrinsicGasArray, gasLimits, gasPrices, err := parseTxs(config, &block)

func parseTxs(config *params.ChainConfig, block *btBlock) ([]sdk.Msg, []uint64, []uint64, []sdk.DecCoins, error) {
	var msgs []sdk.Msg
	var gasLimits []uint64
	var intrinsicGasArray []uint64
	var gasPrices []sdk.DecCoins
	decodebl, _ := block.decode()
	txs := decodebl.Transactions()
	for _, tx := range txs {
		signer := gethTypes.MakeSigner(config, block.Header.Number)
		from, _ := gethTypes.Sender(signer, tx)

		intrinsicGas, err := core.IntrinsicGas(tx.Data(), len(to) == 0, true, true)

*/

func readTestCases(filename string) map[string]*BlockTest {
	testCases := make(map[string]*BlockTest)
	err := ReadJSONFile(filename, &testCases)
	if err != nil {
		panic(err)
	}

	return testCases
}

func ReadJSONFile(fn string, value interface{}) error {
	file, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer file.Close()

	err = readJSON(file, value)
	if err != nil {
		return fmt.Errorf("%s in file %s", err.Error(), fn)
	}
	return nil
}

//func printSender(testCase *BlockTest) {
//	config := tests.Forks[testCase.Json.Network]
//	for _, block := range testCase.Json.Blocks {
//		decodeBlk, _ := block.decode()
//		txs := decodeBlk.Transactions()
//		for _, tx := range txs {
//			signer := types.MakeSigner(config, block.Header.Number)
//			from, _ := types.Sender(signer, tx)
//			fmt.Printf("Sender: %s\n", strings.ToLower(from.String()))
//		}
//	}
//}

/*
type GenesisAlloc map[common.Address]GenesisAccount

type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

type stTransaction struct {
	Nonce    uint64   `json:"nonce"`
	To       string   `json:"to"`
	Data     string   `json:"data"`
	Value    *big.Int `json:"value"`
	GasLimit uint64   `json:"gasLimit"`
	GasPrice *big.Int `json:"gasPrice"`
	V        *big.Int `json:"v"`
	R        *big.Int `json:"r"`
	S        *big.Int `json:"s"`
}

struct block_info {
    evmc_address coinbase;
    int64_t number;
    int64_t timestamp;
    int64_t gas_limit;
    evmc_uint256be difficulty;
    evmc_uint256be chain_id;
};
type btHeader struct {
	Coinbase         common.Address `json:"coinbase"`
	Number           *big.Int `json:"number"`
	Difficulty       *big.Int
	GasLimit         uint64
	Timestamp        uint64 `json:"timestamp"`
}
*/

func printState(state map[common.Address]core.GenesisAccount) {
	addrList := make([]common.Address, 0, len(state))
	for addr := range state {
		addrList = append(addrList, addr)
	}
	sort.Slice(addrList, func(i, j int) bool {
		return bytes.Compare(addrList[i][:], addrList[j][:]) < 0
	})
	for _, addr := range addrList {
		acc := state[addr]
		fmt.Printf(" addr %s\n", strings.ToLower(addr.String()))
		fmt.Printf("  acc_nonce %d\n", acc.Nonce)
		fmt.Printf("  balance %d\n", acc.Balance)
		fmt.Printf("  code %d", len(acc.Code))
		for _, c := range acc.Code {
			fmt.Printf(" %d", c)
		}
		fmt.Printf("\n")
		keyList := make([]common.Hash, 0, len(acc.Storage))
		for k := range acc.Storage {
			keyList = append(keyList, k)
		}
		sort.Slice(keyList, func(i, j int) bool {
			return bytes.Compare(keyList[i][:], keyList[j][:]) < 0
		})
		for _, k := range keyList {
			v := acc.Storage[k]
			fmt.Printf("   kv %s %s\n", k.String(), v.String())
		}
	}
}

func printTx(signer types.Signer, tx *types.Transaction) {
	from, err := types.Sender(signer, tx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("   from %s\n", strings.ToLower(from.String()))
	fmt.Printf("   nonce %d\n", tx.Nonce())
	if tx.To() == nil {
		fmt.Printf("   to 0x0000000000000000000000000000000000000000\n")
	} else {
		fmt.Printf("   to %s\n", strings.ToLower(tx.To().String()))
	}
	fmt.Printf("   value %d\n", tx.Value())
	fmt.Printf("   gasprice %d\n", tx.GasPrice())
	fmt.Printf("   gas %d\n", tx.Gas())
	fmt.Printf("   data %d", len(tx.Data()))
	for _, c := range tx.Data() {
		fmt.Printf(" %d", c)
	}
	fmt.Printf("\n")
}

func printTestCase(testCase *BlockTest) {
	tj := &testCase.Json
	config := tests.Forks[tj.Network]
	fmt.Printf("pre\n")
	printState(tj.Pre)
	fmt.Printf("post\n")
	printState(tj.Post)
	fmt.Printf("blocks\n")
	for i, block := range tj.Blocks {
		fmt.Printf(" block %d\n", i)
		fmt.Printf("  coinbase %s\n", strings.ToLower(block.Header.Coinbase.String()))
		fmt.Printf("  height %d\n", block.Header.Number)
		fmt.Printf("  difficulty %d\n", block.Header.Difficulty)
		fmt.Printf("  gaslimit %d\n", block.Header.GasLimit)
		fmt.Printf("  timestamp %d\n", block.Header.Timestamp)
		decodeBlk, _ := block.decode()
		txs := decodeBlk.Transactions()
		for j, tx := range txs {
			signer := types.MakeSigner(config, block.Header.Number)
			fmt.Printf("  tx %d\n", j)
			printTx(signer, tx)
		}
	}
}

//func (t *StateTest) Run(subtest StateSubtest, vmconfig vm.Config, snapshotter bool) (*state.StateDB, error) {
//	statedb, root, err := t.RunNoVerify(subtest, vmconfig, snapshotter)
//	if err != nil {
//		return statedb, err
//	}
//	post := t.json.Post[subtest.Fork][subtest.Index]
//	// N.B: We need to do this in a two-step process, because the first Commit takes care
//	// of suicides, and we need to touch the coinbase _after_ it has potentially suicided.
//	if root != common.Hash(post.Root) {
//		return statedb, fmt.Errorf("post state root mismatch: got %x, want %x", root, post.Root)
//	}
//	logs := statedb.Logs()
//
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
//	if logs := rlpHash(logs); logs != common.Hash(post.Logs) {
//		return statedb, fmt.Errorf("post state logs hash mismatch: got %x, want %x", logs, post.Logs)
//	}
//	return statedb, nil
//}

func main() {
	filename := os.Args[1]
	testCases := readTestCases(filename)
	nameList := make([]string, 0, len(testCases))
	for testName := range testCases {
		nameList = append(nameList, testName)
	}
	sort.Slice(nameList, func(i, j int) bool {
		return strings.Compare(nameList[i][:], nameList[j][:]) < 0
	})
	for _, testName := range nameList {
		testCase := testCases[testName]
		fmt.Printf("test %s\n", testName)
		printTestCase(testCase)
	}
}
