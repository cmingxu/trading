package main

import (
	"math/big"

	"github.com/xssnick/tonutils-go/address"
)

type Pool struct {
	Addr        *address.Address `json:"addr"`
	Symbol      string           `json:"symbol"`
	Decimals    uint8            `json:"decimals"`
	Name        string           `json:"name"`
	Image       string           `json:"image"`
	Description string           `json:"description"`

	Token0PoolLiquidity *big.Int         `json:"token0_pool_liquidity"`
	Token1PoolLiquidity *big.Int         `json:"token1_pool_liquidity"`
	Token0WalletAddr    *address.Address `json:"token0_wallet_addr"`
	Token1WalletAddr    *address.Address `json:"token1_wallet_addr"`

	LPFee       int `json:"lp_fee"`
	ProtocolFee int `json:"protocol_fee"`
	RefFee      int `json:"ref_fee"`
}
