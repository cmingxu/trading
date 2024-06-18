package main

import (
	"math/big"

	"github.com/xssnick/tonutils-go/address"
)

type Token struct {
	Name          string           `json:"name"`
	Symbol        string           `json:"symbol"`
	Image         string           `json:"image"`
	Description   string           `json:"description"`
	Decimals      uint8            `json:"decimals"`
	Address       *address.Address `json:"address"`
	WalletAddress *address.Address `json:"wallet_address"`
	Amount        *big.Int         `json:"amount"`
}
