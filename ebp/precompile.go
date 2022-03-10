package ebp

import (
	"errors"
	"unsafe"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/vechain/go-ecvrf"
)

//#include <stdint.h>
//#include "../evmwrap/host_bridge/bridge.h"
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

const (
	VRF_VERIFY_GAS uint64 = 5000
)

type VrfVerifyContract struct{}

func (vdfc *VrfVerifyContract) RequiredGas(input []byte) uint64 {
	return VRF_VERIFY_GAS
}

func (vdfc *VrfVerifyContract) Run(input []byte) ([]byte, error) {
	var zeros [32]byte
	if len(input) <= 32+33 {
		return zeros[:], errors.New("input two short")
	}
	// prepare input: abi.encodePacked(alpha/*uint256*/, pubKeyBytes/*33 bytes*/, pi/*variable-length bytes*/)
	alpha := input[0:32]
	pubKeyBytes := input[32 : 32+33]
	pi := input[32+33:]
	pubKey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		return zeros[:], err
	}
	vrf := ecvrf.NewSecp256k1Sha256Tai()
	beta, err := vrf.Verify(pubKey.ToECDSA(), alpha, pi)
	if err != nil {
		return zeros[:], err
	}
	return beta, nil
}

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
	if addr == common.Address([20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x27, 0x13}) {
		contract = &VrfVerifyContract{}
		ok = true
	} else if executor, exist := PredefinedContractManager[addr]; exist {
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
