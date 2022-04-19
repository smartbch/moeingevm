for d in stArgsZeroOneBalance stAttackTest stBadOpcode stBugs stCallCodes stCallCreateCallCodeTest stCallDelegateCodesCallCodeHomestead stCallDelegateCodesHomestead stChainId stChangedEIP150 stCodeCopyTest stCodeSizeLimit stCreate2 stCreateTest stDelegatecallTestHomestead stEIP150Specific stEIP150singleCodeGasPrices stEIP158Specific stExample stExtCodeHash stHomesteadSpecific stInitCodeTest stLogTests stMemExpandingEIP150Calls stMemoryStressTest stMemoryTest stNonZeroCallsTest stZeroCallsTest stRecursiveCreate stRefundTest  stSStoreTest stSelfBalance stShift  stSolidityTest stStackTests stStaticCall stTransitionTest stWalletTest stZeroCallsRevert stZeroKnowledge stZeroKnowledge2 stTransactionTest stSystemOperationsTest stSpecialTest stSLoadTest stRevertTest stReturnDataTest stPreCompiledContracts stPreCompiledContracts2 stQuadraticComplexityTest stStaticCall stTimeConsuming stRandom stRandom2; do
	for f in ../../../testdata/evm/$d/*.txt; do
		echo "f=$f"
		gawk '$1=="test" {print "./run_test_case $f "$2}' $f
	done
done



