package main

type Message struct {
	AccountId string `json:"account_id"`
	Hash      string `json:"tx_hash"`
	Lt        uint64 `json:"lt"`
}
