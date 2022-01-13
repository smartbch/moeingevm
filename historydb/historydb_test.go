package historydb

import (
	"testing"
)

// go test -c .
// ./historydb.test -test.run TestStep1
// ./historydb.test -test.run TestStep2

func TestStep1(t *testing.T) {
	generateHisDb("./modb", "./hisdb", 100000)
}

func TestStep2(t *testing.T) {
	runTestcases("./hisdb", "http://127.0.0.1:8545")
}
