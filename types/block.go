package types

import "encoding/binary"

//go:generate msgp

//BLOCK - A block object, or null when no block was found
type Block struct {
	Number           int64      `msg:"num"`     //the block number. Null when the returned block is the pending block.
	Hash             [32]byte   `msg:"hash"`    //32 Bytes - hash of the block. Null when the returned block is the pending block.
	ParentHash       [32]byte   `msg:"parent"`  //32 Bytes - hash of the parent block.
	LogsBloom        [256]byte  `msg:"bloom"`   // 256 Bytes - the bloom filter for the logs of the block. Null when the returned block is the pending block.
	TransactionsRoot [32]byte   `msg:"troot"`   // 32 Bytes - the root of the transaction trie of the block.
	StateRoot        [32]byte   `msg:"sroot"`   //32 Bytes - the root of the final state trie of the block.
	Miner            [20]byte   `msg:"miner"`   //20 Bytes - the address of the beneficiary to whom the mining rewards were given.
	Size             int64      `msg:"size"`    //integer the size of this block in bytes.
	GasUsed          uint64     `msg:"gasused"` //the total used gas by all transactions in this block.
	Timestamp        int64      `msg:"time"`    //the unix timestamp for when the block was collated.
	Transactions     [][32]byte `msg:"txs"`     //Array - Array of transaction objects, or 32 Bytes transaction hashes depending on the last given parameter.
	//GasLimit         uint64 //the maximum gas allowed in this block.
	//Nonce: 8 Bytes - hash of the generated proof-of-work. Null when the returned block is the pending block.
	//Sha3Uncles: 32 Bytes - SHA3 of the uncles data in the block.
	//ReceiptsRoot: 32 Bytes - the root of the receipts trie of the block.
	//Difficulty: integer of the difficulty for this block.
	//TotalDifficulty: integer of the total difficulty of the chain until this block.
	//ExtraData: the "extra data" field of this block.
	//Uncles: an Array of uncle hashes.
}

//func BuildBlockFromTM(tmBlock *tmtypes.Block) *Block {
//	block := &Block{
//		Number: tmBlock.Height,
//	}
//	copy(block.Hash[:], tmBlock.Hash())
//	copy(block.ParentHash[:], tmBlock.LastBlockID.Hash)
//	// TODO
//	return block
//}

func (blk *Block) SerializeBasicInfo() []byte {
	bz := make([]byte, 8+8+8, 8+8+8+32+32+20)
	binary.LittleEndian.PutUint64(bz[0:8], uint64(blk.Number))
	binary.LittleEndian.PutUint64(bz[8:16], uint64(blk.Timestamp))
	binary.LittleEndian.PutUint64(bz[16:24], uint64(blk.Size))
	bz = append(bz, blk.ParentHash[:]...)
	bz = append(bz, blk.Hash[:]...)
	bz = append(bz, blk.Miner[:]...)
	return bz
}

func (blk *Block) FillBasicInfo(bz []byte) {
	if len(bz) != 8+8+8+32+32+20 {
		panic("Invalid Length")
	}
	start := 0
	blk.Number = int64(binary.LittleEndian.Uint64(bz[start : start+8]))
	start += 8
	blk.Timestamp = int64(binary.LittleEndian.Uint64(bz[start : start+8]))
	start += 8
	blk.Size = int64(binary.LittleEndian.Uint64(bz[start : start+8]))
	start += 8
	copy(blk.ParentHash[:], bz[start:start+32])
	start += 32
	copy(blk.Hash[:], bz[start:start+32])
	start += 32
	copy(blk.Miner[:], bz[start:])
}

/*
eth_accounts null
eth_blockNumber simple
eth_call main
eth_chainId simple
eth_estimateGas main
eth_gasPrice main
eth_getBalance main
eth_getBlockByHash external
eth_getBlockByNumber external
eth_getBlockTransactionCountByHash external
eth_getBlockTransactionCountByNumber external
eth_getCode main
eth_getLogs external
eth_getStorageAt main
eth_getTransactionByBlockHashAndIndex external
eth_getTransactionByBlockNumberAndIndex external
eth_getTransactionByHash external
eth_getTransactionCount main //Use the pending tag to get the next account nonce not used by any pending transactions.
eth_getTransactionReceipt external
eth_getUncleByBlockHashAndIndex null
eth_getUncleByBlockNumberAndIndex null
eth_getUncleCountByBlockHash null
eth_getUncleCountByBlockNumber null
eth_getWork null
eth_hashrate null
eth_mining null
eth_protocolVersion simple
eth_sendRawTransaction main
eth_submitWork null
eth_syncing TODO
net_listening
net_peerCount
net_version
web3_clientVersion
Filter Methods
eth_newFilter external
eth_newBlockFilter external
eth_getFilterChanges external
eth_uninstallFilter external
Rate Limits
WSS
Introduction
eth_subscribe external
eth_unsubscribe external
*/
