package ebp

import (
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	modbtypes "github.com/smartbch/moeingdb/types"

	"github.com/smartbch/moeingevm/types"
)

type TxExecutor interface {
	//step 1: for deliverTx, collect block txs in engine.txList
	CollectTx(tx *gethtypes.Transaction)
	//step 2: for commit, check sig, insert regular txs standbyTxQ
	Prepare(reorderSeed int64, minGasPrice uint64) (touchedAddrs map[common.Address]int)
	//step 3: for postCommit, parallel execute tx in standbyTxQ
	Execute(currBlock *types.BlockInfo)

	//set context
	SetContext(ctx *types.Context)
	Context() *types.Context

	//collect infos, not thread safe
	CollectTxsCount() int
	CommittedTxs() []*types.Transaction
	CommittedTxIds() [][32]byte
	CommittedTxsForMoDB() []modbtypes.Tx
	GasUsedInfo() (gasUsed uint64, gasRefund, gasFee uint256.Int)
	StandbyQLen() int
}
