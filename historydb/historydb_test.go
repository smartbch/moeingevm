package historydb

import (
	"os"
	"testing"
)

// go test -c .
// HISTORYDBTEST=YES ./historydb.test -test.run TestStep0
// HISTORYDBTEST=YES ./historydb.test -test.run TestStep1
// HISTORYDBTEST=YES ./historydb.test -test.run TestStep2

func TestStep0(t *testing.T) {
	if os.Getenv("HISTORYDBTEST") != "YES" {
		return
	}
	testTheOnlyTxInBlocks("./modb", "http://127.0.0.1:8545", 100000)
}

func TestStep1(t *testing.T) {
	if os.Getenv("HISTORYDBTEST") != "YES" {
		return
	}
	generateHisDb("./modb", "./hisdb", 100000)
}

func TestStep2(t *testing.T) {
	if os.Getenv("HISTORYDBTEST") != "YES" {
		return
	}
	runTestcases("./hisdb", "http://127.0.0.1:8545")
}
