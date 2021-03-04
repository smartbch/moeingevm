package types

import (
	"github.com/ethereum/go-ethereum/common"
)

type SystemContractExecutor interface {
	IsSystemContract(addr common.Address) bool
	Execute(context Context, tx *TxToRun) (status int, logs []EvmLog, gasUsed uint64)
}

