for d in stArgsZeroOneBalance stAttackTest stBadOpcode stBugs stCallCodes stCallCreateCallCodeTest stCallDelegateCodesCallCodeHomestead stCallDelegateCodesHomestead stChainId stChangedEIP150 stCodeCopyTest stCodeSizeLimit stCreate2 stCreateTest stDelegatecallTestHomestead stEIP150Specific stEIP150singleCodeGasPrices stEIP158Specific stExample stExtCodeHash stHomesteadSpecific stInitCodeTest stLogTests stMemExpandingEIP150Calls stMemoryStressTest stMemoryTest stNonZeroCallsTest stZeroCallsTest stRecursiveCreate stRefundTest  stSStoreTest stSelfBalance stShift  stSolidityTest stStackTests stStaticCall stTransitionTest stWalletTest stZeroCallsRevert stZeroKnowledge stZeroKnowledge2 stTransactionTest stSystemOperationsTest stSpecialTest stSLoadTest stRevertTest stReturnDataTest stPreCompiledContracts stPreCompiledContracts2 stQuadraticComplexityTest stStaticCall stTimeConsuming stRandom stRandom2; do
	#NODIASM=1 NOTRACE=1 NOSTACK=1 ./run_test_dir ../../../testdata/evm/$d
	DISABLE_ANALYSIS_CACHE=1 ./run_test_dir ../../../testdata/evm/$d
	if [ $? -ne 0 ]; then
		break
	fi
done



