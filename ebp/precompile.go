package ebp

import (
	"unsafe"

	"github.com/ethereum/go-ethereum/core/vm"
)

//#include <stdint.h>
import "C"

//var PrecompiledContractsIstanbul map[common.Address]PrecompiledContract
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
	*output_size = 0
	addr := toAddress(contract_addr)
	contract, ok := vm.PrecompiledContractsIstanbul[addr]
	if executor, ok := PredefinedContractManager[addr]; ok {
		contract = executor
		ok = true
	}
	if !ok {
		*ret_value = 0
		*out_of_gas = 0
		return
	}
	input := C.GoBytes(input_ptr, input_size)
	gasRequired := C.uint64_t(contract.RequiredGas(input))
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
	if size > SMALL_BUF_SIZE { // limit the copied data to prevent overflow
		size = SMALL_BUF_SIZE
	}
	*output_size = C.int(size)
	for i := 0; i < size; i++ {
		output_ptr.data[i] = C.uint8_t(output[i])
	}
	*ret_value = 1
	*out_of_gas = 0
}
