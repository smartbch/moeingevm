package ebp

import (
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	modbtypes "github.com/smartbch/moeingdb/types"

	"github.com/smartbch/moeingevm/types"
)

type TxExecutor interface {
	SetAotParam(aotDir string, aotReloadInterval int64)

	//step 1: for deliverTx, collect block txs in engine.txList
	CollectTx(tx *gethtypes.Transaction)
	//step 2: for commit, check sig, insert regular txs standbyTxQ
	Prepare(reorderSeed int64, minGasPrice, maxTxGasLimit uint64) Frontier
	//step 3: for postCommit, parallel execute tx in standbyTxQ
	Execute(currBlock *types.BlockInfo)

	//set context
	SetContext(ctx *types.Context)
	Context() *types.Context

	//collect infos, not thread safe
	CollectedTxsCount() int
	CommittedTxs() []*types.Transaction
	CommittedTxIds() [][32]byte
	CommittedTxsForMoDB() []modbtypes.Tx
	GasUsedInfo() (gasUsed uint64, feeRefund, gasFee uint256.Int)
	StandbyQLen() int
}

type Frontier interface {
	GetLatestNonce(addr common.Address) (nonce uint64, exist bool)
	SetLatestNonce(addr common.Address, newNonce uint64)
	GetLatestBalance(addr common.Address) (balance *uint256.Int, exist bool)
	SetLatestBalance(addr common.Address, balance *uint256.Int)
	GetLatestTotalGas(addr common.Address) (gas uint64, exist bool)
	SetLatestTotalGas(addr common.Address, gas uint64)
}
