package main

import (
	//"fmt"
	"unsafe"

	//"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

//#include <stdint.h>
//#include "../host_bridge/bridge.h"
import "C"

//var PrecompiledContractsIstanbul = map[common.Address]PrecompiledContract{
//func (c *ecrecover) RequiredGas(input []byte) uint64 {
//func (c *ecrecover) Run(input []byte) ([]byte, error) {
//byte{1}): &ecrecover{},
//byte{2}): &sha256hash{},
//byte{3}): &ripemd160hash{},
//byte{4}): &dataCopy{},
//byte{5}): &bigModExp{},
//byte{6}): &bn256AddIstanbul{},
//byte{7}): &bn256ScalarMulIstanbul{},
//byte{8}): &bn256PairingIstanbul{},
//byte{9}): &blake2F{},

//export call_precompiled_contract
func call_precompiled_contract(contract_addr *evmc_address,
	input_ptr unsafe.Pointer,
	input_size C.int,
	gas_left *C.uint64_t,
	ret_value *C.int,
	out_of_gas *C.int,
	output_ptr *small_buffer,
	output_size *C.int) {
	addr := toArr20(contract_addr)
	contract, ok := vm.PrecompiledContractsIstanbul[addr]
	if !ok {
		*ret_value = 0
		*out_of_gas = 0
		return
	}
	input := C.GoBytes(input_ptr, input_size)
	gasRequired := C.uint64_t(contract.RequiredGas(input))
	//fmt.Printf("Why gasRequired %d gas_left %d input_size%d %#v\n", gasRequired, *gas_left, input_size, input)
	if gasRequired > *gas_left {
		*ret_value = 0
		*out_of_gas = 1
		*gas_left = 0
		return
	}
	*gas_left -= gasRequired
	output, err := contract.Run(input)
	if err != nil {
		*ret_value = 0
		*out_of_gas = 0
		return
	}
	size := len(output)
	if size > 256 {
		size = 256
	}
	*output_size = C.int(size)
	//fmt.Printf("PRECOM %s %d  %d", common.Address(addr).String(), len(output), size)
	for i := 0; i < size && i < C.SMALL_BUF_SIZE; i++ {
		output_ptr.data[i] = C.uint8_t(output[i])
		//fmt.Printf("%x", int(output[i]));
	}
	//fmt.Printf("\n");
	*ret_value = 1
	*out_of_gas = 0
	//return
}
