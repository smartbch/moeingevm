package types

//go:generate msgp

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

type InternalTxCall struct {
	/** The kind of the call. For zero-depth calls ::EVMC_CALL SHOULD be used. */
	Kind int `msg:"kind"`

	/**
	* Additional flags modifying the call execution behavior.
	* In the current version the only valid values are ::EVMC_STATIC or 0.
	 */
	Flags uint32 `msg:"flags"`

	/** The call depth. */
	Depth int32 `msg:"depth"`

	/** The amount of gas for message execution. */
	Gas int64

	/** The callee of the transaction. */
	Destination [20]byte `msg:"destination"`

	/** The caller of the transaction. */
	Sender [20]byte `msg:"sender"`

	/** the input data.  */
	Input []byte `msg:"input"`

	/**
	* The amount of BCH transferred with the message.
	 */
	Value [32]byte `msg:"value"`
}

type InternalTxReturn struct {
	/** The execution status code. */
	StatusCode int `msg:"statusCode"`

	/**
	* The amount of gas left after the execution.
	* If StatusCode is neither ::EVMC_SUCCESS nor ::EVMC_REVERT
	* the value MUST be 0.
	 */
	GasLeft int64 `msg:"gasLeft"`

	/**  the output data.  */
	Output []byte `msg:"output"`

	/**
	* The address of the contract created by create instructions.
	*
	* This field has valid value only if:
	* - it is a result of the Host method evmc_host_interface::call
	* - and the result describes successful contract creation
	*   (StatusCode is ::EVMC_SUCCESS).
	* In all other cases the address MUST be null bytes.
	 */
	CreateAddress [20]byte `msg:"createAddress"`
}

type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address [20]byte `msg:"address"`
	// list of topics provided by the contract.
	Topics [][32]byte `msg:"topics"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `msg:"data"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `msg:"blockNumber"`
	// hash of the transaction
	TxHash [32]byte `msg:"transactionHash"`
	// index of the transaction in the block
	TxIndex uint `msg:"transactionIndex"`
	// hash of the block in which the transaction was included
	BlockHash [32]byte `msg:"blockHash"`
	// index of the log in the block
	Index uint `msg:"logIndex"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `msg:"removed"`
}

type CreationCounterRWOp struct {
	Lsb     uint8  `msg:"lsb"`
	Counter uint64 `msg:"counter"`
}
type AccountRWOp struct {
	Addr    [20]byte `msg:"addr"`
	Account []byte   `msg:"account"`
}
type BytecodeRWOp struct {
	Addr     [20]byte `msg:"addr"`
	Bytecode []byte   `msg:"bytecode"`
}
type StorageRWOp struct {
	Seq   uint64 `msg:"seq"`
	Key   string `msg:"key"`
	Value []byte `msg:"value"`
}
type BlockHashOp struct {
	Height uint64   `msg:"height"`
	Hash   [32]byte `msg:"hash"`
}

type ReadWriteLists struct {
	CreationCounterRList []CreationCounterRWOp `msg:"creationcounter_rlist"`
	CreationCounterWList []CreationCounterRWOp `msg:"creationcounter_wlist"`
	AccountRList         []AccountRWOp         `msg:"account_rlist"`
	AccountWList         []AccountRWOp         `msg:"account_wlist"`
	BytecodeRList        []BytecodeRWOp        `msg:"bytecode_rlist"`
	BytecodeWList        []BytecodeRWOp        `msg:"bytecode_wlist"`
	StorageRList         []StorageRWOp         `msg:"storage_rlist"`
	StorageWList         []StorageRWOp         `msg:"storage_wlist"`
	BlockHashList        []BlockHashOp         `msg:"blockhash_list"`
}

//logs are objects with following params (Using types.Log is OK):
// removed: true when the log was removed, due to a chain reorganization. false if it's a valid log.
// logIndex: integer of the log index position in the block. null when its pending log.
// transactionIndex: integer of the transactions index position log was created from. null when its pending log.
// transactionHash: 32 Bytes - hash of the transactions this log was created from. null when its pending log.
// blockHash: 32 Bytes - hash of the block where this log was in. null when its pending. null when its pending log.
// blockNumber: the block number where this log was in. null when its pending. null when its pending log.
// address: 20 Bytes - address from which this log originated.
// data: contains one or more 32 Bytes non-indexed arguments of the log.
// topics: Array of 0 to 4 32 Bytes of indexed log arguments. (In solidity: The first topic is the hash of the signature of the event (e.g. Deposit(address,bytes32,uint256)), except you declared the event with the anonymous specifier.)

//TRANSACTION - A transaction object, or null when no transaction was found
type Transaction struct {
	Hash              [32]byte  `msg:"hash"`         //32 Bytes - hash of the transaction.
	TransactionIndex  int64     `msg:"index"`        //integer of the transactions index position in the block. null when its pending.
	Nonce             uint64    `msg:"nonce"`        //the number of transactions made by the sender prior to this one.
	BlockHash         [32]byte  `msg:"block"`        //32 Bytes - hash of the block where this transaction was in. null when its pending.
	BlockNumber       int64     `msg:"height"`       //block number where this transaction was in. null when its pending.
	From              [20]byte  `msg:"from"`         //20 Bytes - address of the sender.
	To                [20]byte  `msg:"to"`           //20 Bytes - address of the receiver. null when its a contract creation transaction.
	Value             [32]byte  `msg:"value"`        //value transferred in Wei.
	GasPrice          [32]byte  `msg:"gasprice"`     //gas price provided by the sender in Wei.
	Gas               uint64    `msg:"gas"`          //gas provided by the sender.
	Input             []byte    `msg:"input"`        //the data send along with the transaction.
	CumulativeGasUsed uint64    `msg:"cgasused"`     // the total amount of gas used when this transaction was executed in the block.
	GasUsed           uint64    `msg:"gasused"`      //the amount of gas used by this specific transaction alone.
	ContractAddress   [20]byte  `msg:"contractaddr"` //20 Bytes - the contract address created, if the transaction was a contract creation, otherwise - null.
	Logs              []Log     `msg:"logs"`         //Array - Array of log objects, which this transaction generated.
	LogsBloom         [256]byte `msg:"bloom"`        //256 Bytes - Bloom filter for light clients to quickly retrieve related logs.
	Status            uint64    `msg:"status"`       //tx execute result: ReceiptStatusFailed or ReceiptStatusSuccessful
	StatusStr         string    `msg:"statusstr"`    //tx execute result explained
	OutData           []byte    `msg:"outdata"`      //the output data from the transaction
	//PostState  []byte  //look at Receipt.PostState

	InternalTxCalls   []InternalTxCall   `msg:"itxcalls"`
	InternalTxReturns []InternalTxReturn `msg:"itxreturns"`

	RwLists *ReadWriteLists `msg:"rwlist"`
}

//TRANSACTION RECEIPT - A transaction receipt object, or null when no receipt was found:
// transactionHash: 32 Bytes - hash of the transaction.
// transactionIndex: integer of the transactions index position in the block.
// blockHash: 32 Bytes - hash of the block where this transaction was in.
// blockNumber: block number where this transaction was in.
// from: 20 Bytes - address of the sender.
// to: 20 Bytes - address of the receiver. Null when the transaction is a contract creation transaction.
// *cumulativeGasUsed: the total amount of gas used when this transaction was executed in the block.
// *gasUsed: the amount of gas used by this specific transaction alone.
// *contractAddress: 20 Bytes - the contract address created, if the transaction was a contract creation, otherwise - null.
// *logs: Array - Array of log objects, which this transaction generated.
// *logsBloom: 256 Bytes - Bloom filter for light clients to quickly retrieve related logs.
