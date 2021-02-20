package types

import "errors"

var (
	ErrAccNotFound         = errors.New("account not found")
	ErrCodeNotFound        = errors.New("code not found")
	ErrBadAccData          = errors.New("bad account data")
	ErrBadNonce            = errors.New("bad nonce")
	ErrBadGasPrice         = errors.New("bad gas price")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrBlockNotFound       = errors.New("block not found")
	ErrTxNotFound          = errors.New("tx not found")
	ErrNoFromAddr          = errors.New("missing from address")
	ErrInvalidHeight       = errors.New("invalid height")
)
