package main

import (
	"math/big"

	"github.com/xssnick/tonutils-go/address"
)

type Trading struct {
	Action      string           `json:"action"`
	Pool        *Pool            `json:"pool"`
	TxHash      string           `json:"tx_hash"`
	AccountId   string           `json:"account_id"`
	TokenWallet *address.Address `json:"token_wallet"`
	From        *address.Address `json:"from"`
	Lt          uint64           `json:"lt"`
	Balance     *big.Int         `json:"balance"`
	Amount      *big.Int         `json:"amount"`
	HasRef      bool             `json:"has_ref"`
	MinOut      *big.Int         `json:"min_out"`
	RefAddr     *address.Address `json:"ref_addr"`
	Token0      *Token           `json:"token0"`
	Token1      *Token           `json:"token1"`
}
