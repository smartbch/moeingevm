package ebptests

import (
	"os"
	"testing"

	tc "github.com/smartbch/moeingevm/evmwrap/testcase"
)

// go test -c -coverpkg github.com/smartbch/moeingevm/ebp .
// NOINSTLOG=1 NODIASM=1 NOSTACK=1 ./ebptests.test -test.coverprofile a.out

func Test1(t *testing.T) {
	dirList := make([]string, 0, 1000)
	dirList = append(dirList, "stExample")
	if os.Getenv("RUN_ALL_EBP_TESTS") == "YES" {
		dirList = append(dirList, "stArgsZeroOneBalance")
		dirList = append(dirList, "stAttackTest")
		dirList = append(dirList, "stBadOpcode")
		dirList = append(dirList, "stBugs")
		dirList = append(dirList, "stCallCodes")
		dirList = append(dirList, "stCallCreateCallCodeTest")
		dirList = append(dirList, "stCallDelegateCodesCallCodeHomestead")
		dirList = append(dirList, "stCallDelegateCodesHomestead")
		dirList = append(dirList, "stChainId")
		dirList = append(dirList, "stChangedEIP150")
		dirList = append(dirList, "stCodeCopyTest")
		dirList = append(dirList, "stCodeSizeLimit")
		dirList = append(dirList, "stCreate2")
		dirList = append(dirList, "stCreateTest")
		dirList = append(dirList, "stDelegatecallTestHomestead")
		dirList = append(dirList, "stEIP150Specific")
		dirList = append(dirList, "stEIP150singleCodeGasPrices")
		dirList = append(dirList, "stEIP158Specific")
		dirList = append(dirList, "stExtCodeHash")
		dirList = append(dirList, "stHomesteadSpecific")
		dirList = append(dirList, "stInitCodeTest")
		dirList = append(dirList, "stLogTests")
		dirList = append(dirList, "stMemExpandingEIP150Calls")
		dirList = append(dirList, "stMemoryStressTest")
		dirList = append(dirList, "stMemoryTest")
		dirList = append(dirList, "stNonZeroCallsTest")
		dirList = append(dirList, "stZeroCallsTest")
		dirList = append(dirList, "stRecursiveCreate")
		dirList = append(dirList, "stRefundTest")
		dirList = append(dirList, "stSStoreTest")
		dirList = append(dirList, "stSelfBalance")
		dirList = append(dirList, "stShift")
		dirList = append(dirList, "stSolidityTest")
		dirList = append(dirList, "stStackTests")
		dirList = append(dirList, "stStaticCall")
		dirList = append(dirList, "stTransitionTest")
		dirList = append(dirList, "stWalletTest")
		dirList = append(dirList, "stZeroCallsRevert")
		dirList = append(dirList, "stZeroKnowledge")
		dirList = append(dirList, "stZeroKnowledge2")
		dirList = append(dirList, "stTransactionTest")
		dirList = append(dirList, "stSystemOperationsTest")
		dirList = append(dirList, "stSpecialTest")
		dirList = append(dirList, "stSLoadTest")
		dirList = append(dirList, "stRevertTest")
		dirList = append(dirList, "stReturnDataTest")
		dirList = append(dirList, "stPreCompiledContracts")
		dirList = append(dirList, "stPreCompiledContracts2")
		dirList = append(dirList, "stQuadraticComplexityTest")
		dirList = append(dirList, "stStaticCall")
		dirList = append(dirList, "stTimeConsuming")
		dirList = append(dirList, "stRandom")
		dirList = append(dirList, "stRandom2")
	}
	for _, dir := range dirList {
		dir = "../../../testdata/evm/" + dir
		tc.RunOneDir([]string{"run_test_dir", dir}, true, runTestCase)
	}
}
