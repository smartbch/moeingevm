package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/tendermint/tendermint/crypto"
)

// BondStatus is the status of a validator
type BondStatus uint64

const (
	//unbonded = 0x00
	//bonded   = 0x01

	BondStatusUnbonded = "Unbonded"
	BondStatusBonded   = "Bonded"
)

// String implements the Stringer interface for BondStatus.
func (b BondStatus) String() string {
	switch b {
	case 0x00:
		return BondStatusUnbonded
	case 0x01:
		return BondStatusBonded
	default:
		panic("invalid bond status")
	}
}

type ValAddress []byte

// Description - description fields for a validator
type Description struct {
	Moniker         string `json:"moniker" yaml:"moniker"`                   // name
	Identity        string `json:"identity" yaml:"identity"`                 // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website" yaml:"website"`                   // optional website link
	SecurityContact string `json:"security_contact" yaml:"security_contact"` // optional security contact info
	Details         string `json:"details" yaml:"details"`                   // optional details
}

// NewDescription returns a new Description with the provided values.
func NewDescription(moniker, identity, website, securityContact, details string) Description {
	return Description{
		Moniker:         moniker,
		Identity:        identity,
		Website:         website,
		SecurityContact: securityContact,
		Details:         details,
	}
}

type Validator struct {
	OperatorAddress ValAddress    `json:"operator_address" yaml:"operator_address"` // address of the validator's operator; bech encoded in JSON
	ConsPubKey      crypto.PubKey `json:"consensus_pubkey" yaml:"consensus_pubkey"` // the consensus public key of the validator; bech encoded in JSON
	//Jailed                  bool           `json:"jailed" yaml:"jailed"`                           // has the validator been jailed from bonded status?
	Status      BondStatus  `json:"status" yaml:"status"`           // validator status (bonded/unbonded)
	Tokens      *big.Int    `json:"tokens" yaml:"tokens"`           // delegated tokens
	Description Description `json:"description" yaml:"description"` // description terms for the validator
}

func (v Validator) MarshalJSON() ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON unmarshals the validator from JSON using Bech32
func (v *Validator) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, v)
}

// String returns a human readable string representation of a validator.
func (v Validator) String() string {
	return fmt.Sprintf(`Validator
  Operator Address:           %s
  Validator Consensus Pubkey: %s
  Status : 		%s
  Tokens:                     %s
  Description:                %s`,
		v.OperatorAddress,
		v.ConsPubKey,
		v.Status.String(),
		v.Tokens,
		v.Description)
}

// Validators is a collection of Validator
type Validators []Validator

func (v Validators) String() (out string) {
	for _, val := range v {
		out += val.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// Sort Validators sorts validator array in ascending operator address order
func (v Validators) Sort() {
	sort.Sort(v)
}

// Implements sort interface
func (v Validators) Len() int {
	return len(v)
}

// Implements sort interface
func (v Validators) Less(i, j int) bool {
	return bytes.Compare(v[i].OperatorAddress, v[j].OperatorAddress) == -1
}

// Implements sort interface
func (v Validators) Swap(i, j int) {
	it := v[i]
	v[i] = v[j]
	v[j] = it
}
