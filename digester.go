package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
	"github.com/xssnick/tonutils-go/address"
)

const ANTON_TX = "http://<ANTON_ADDR>:8080/api/v0/transactions?hash="
const ACCOUNT_TPL = "http://ANTON_ADDR>:8080/api/v0/accounts?address=%s&latest=true"

type Digester struct {
	MessageChan chan *Message
	TradingChan chan *Trading
}

func NewDigester() *Digester {
	return &Digester{
		MessageChan: make(chan *Message, 10),
		TradingChan: make(chan *Trading, 10),
	}
}

var poolCache = sync.Map{}
var jettonMasterCache = sync.Map{}
var jettonMasterAddrCache = sync.Map{}

func (d *Digester) MakePool(uri string) (*Pool, error) {
	log.Debug().Str("uri", uri).Msg("Getting pool")

	if v, ok := poolCache.Load(uri); ok {
		return v.(*Pool), nil
	}

	resp, err := http.Get(uri)
	if err != nil {
		log.Err(err).Str("uri", uri).Msg("Failed to get pool")
		return nil, err
	}
	defer resp.Body.Close()

	p := &Pool{}
	err = json.NewDecoder(resp.Body).Decode(p)
	if err != nil {
		log.Err(err).Str("uri", uri).Msg("Failed to decode pool")
		return nil, err
	}
	poolCache.Store(uri, p)
	return p, nil
}

func (d *Digester) MakeJettonMaster(jettonWalletAddress *address.Address, token *Token) error {
	if v, ok := jettonMasterAddrCache.Load(jettonWalletAddress.String()); ok {
		token.Address = v.(*address.Address)
	}

	if v, ok := jettonMasterCache.Load(jettonWalletAddress.String()); ok {
		json.Unmarshal(v.([]byte), token)
		return nil
	}

	jettonResp, err := http.Get(fmt.Sprintf(ACCOUNT_TPL, jettonWalletAddress.String()))
	if err != nil {
		log.Err(err).Str("wallet_address", jettonWalletAddress.String()).Msg("Failed to get jetton master")
		return err
	}
	defer jettonResp.Body.Close()
	jettonRes, err := io.ReadAll(jettonResp.Body)
	if err != nil {
		log.Err(err).Str("wallet_address", jettonWalletAddress.String()).Msg("Failed to read jetton master")
		return err
	}

	jettonMasterAddress, err := address.ParseAddr(gjson.Get(string(jettonRes), "results.0.executed_get_methods.jetton_wallet.0.returns.2").String())
	if err != nil {
		log.Err(err).Str("wallet_address", jettonWalletAddress.String()).Msg("Failed to parse jetton master")
		return err
	}

	token.Address = jettonMasterAddress
	jettonMasterAddrCache.Store(jettonWalletAddress.String(), jettonMasterAddress)

	jettonMasterResp, err := http.Get(fmt.Sprintf(ACCOUNT_TPL, jettonMasterAddress.String()))
	if err != nil {
		log.Err(err).Str("wallet_address", jettonMasterAddress.String()).Msg("Failed to get jetton master")
		return err
	}
	defer jettonMasterResp.Body.Close()
	jettonMasterRes, err := io.ReadAll(jettonMasterResp.Body)
	if err != nil {
		log.Err(err).Str("wallet_address", jettonMasterAddress.String()).Msg("Failed to read jetton master")
		return err
	}

	contentUrl := gjson.Get(string(jettonMasterRes), "results.0.content_uri").String()
	if contentUrl != "" {
		contentResp, err := http.Get(contentUrl)
		if err != nil {
			log.Err(err).Str("content_url", contentUrl).Msg("Failed to get jetton master content")
			return err
		}
		defer contentResp.Body.Close()

		contentRes, err := io.ReadAll(contentResp.Body)
		if err != nil {
			log.Err(err).Str("content_url", contentUrl).Msg("Failed to read jetton master content")
			return err
		}

		// Store jetton master in cache
		jettonMasterCache.Store(jettonWalletAddress.String(), contentRes)
		json.Unmarshal(contentRes, token)
	}

	contentName := gjson.Get(string(jettonMasterRes), "results.0.content_name").String()
	contentImage := gjson.Get(string(jettonMasterRes), "results.0.content_image").String()
	contentDescription := gjson.Get(string(jettonMasterRes), "results.0.content_description").String()
	if contentName != "" && contentImage != "" && contentDescription != "" {
		token.Name = contentName
		token.Image = contentImage
		token.Description = contentDescription

		content := `{"name":"` + contentName + `","image":"` + contentImage + `","description":"` + contentDescription + `"}`
		jettonMasterCache.Store(jettonWalletAddress.String(), []byte(content))
	}

	return nil

}

func (d *Digester) MakeTrading(m *Message) {
START:
	antonTx, err := http.Get(ANTON_TX + m.Hash)
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to get Anton tx")
		return
	}

	defer antonTx.Body.Close()
	res, err := io.ReadAll(antonTx.Body)
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to read Anton tx")
		return
	}

	if gjson.Get(string(res), "total").Int() == 0 {
		time.Sleep(5 * time.Second)
		log.Info().Str("tx_hash", m.Hash).Msg("Anton tx not found, retrying")
		goto START
	}

	p, err := d.MakePool(gjson.Get(string(res), "results.0.account.content_uri").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to get pool")
		return
	}

	p.Addr, err = address.ParseAddr(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_router.0.returns.0").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse pool address")
		return
	}

	p.Token0WalletAddr, err = address.ParseAddr(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.2").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token0 wallet address")
		return
	}

	p.Token1WalletAddr, err = address.ParseAddr(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.3").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token1 wallet address")
		return
	}

	var ok bool
	p.Token0PoolLiquidity, ok = new(big.Int).SetString(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.0").String(), 10)
	if !ok {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token0 amount")
		return
	}
	p.Token1PoolLiquidity, ok = new(big.Int).SetString(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.1").String(), 10)
	if !ok {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token1 amount")
		return
	}

	p.LPFee = int(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.4").Int())
	p.ProtocolFee = int(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.5").Int())
	p.RefFee = int(gjson.Get(string(res), "results.0.account.executed_get_methods.stonfi_pool.0.returns.6").Int())

	t := &Trading{
		Pool:      p,
		TxHash:    m.Hash,
		AccountId: m.AccountId,
		Lt:        m.Lt,
	}
	t.Amount, ok = new(big.Int).SetString(gjson.Get(string(res), "results.0.in_msg.data.amount").String(), 10)
	t.MinOut, ok = new(big.Int).SetString(gjson.Get(string(res), "results.0.in_msg.data.min_out").String(), 10)
	t.HasRef = gjson.Get(string(res), "results.0.in_msg.data.has_ref").Bool()
	if t.HasRef {
		t.RefAddr, err = address.ParseAddr(gjson.Get(string(res), "results.0.in_msg.data.ref_body.ref_address").String())
		if err != nil {
			log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse ref address")
			return
		}
	}
	t.TokenWallet, err = address.ParseAddr(gjson.Get(string(res), "results.0.in_msg.data.token_wallet").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse account wallet")
	}

	t.From, err = address.ParseAddr(gjson.Get(string(res), "results.0.in_msg.data.from_user").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse account address")
		return
	}

	t.Balance, ok = new(big.Int).SetString(gjson.Get(string(res), "results.0.account.balance").String(), 10)

	paytoMsgIndex := 0
	if t.HasRef {
		paytoMsgIndex = 1
	}

	payToResult := gjson.Get(string(res), fmt.Sprintf("results.0.out_msg.%d.data", paytoMsgIndex))

	t.Token0 = &Token{}
	t.Token0.Amount, ok = new(big.Int).SetString(payToResult.Get("ref_coins_data.amount_0_out").String(), 10)
	if !ok {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token0 amount")
		return
	}

	t.Token0.WalletAddress, err = address.ParseAddr(payToResult.Get("ref_coins_data.token_0_address").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token0 address")
		return
	}

	t.Token1 = &Token{}
	t.Token1.Amount, ok = new(big.Int).SetString(payToResult.Get("ref_coins_data.amount_1_out").String(), 10)
	if !ok {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token1 amount")
		return
	}
	t.Token1.WalletAddress, err = address.ParseAddr(payToResult.Get("ref_coins_data.token_1_address").String())
	if err != nil {
		log.Err(err).Str("tx_hash", m.Hash).Msg("Failed to parse token0 address")
		return
	}

	d.MakeJettonMaster(t.Token0.WalletAddress, t.Token0)
	d.MakeJettonMaster(t.Token1.WalletAddress, t.Token1)

	d.TradingChan <- t
}

func (d *Digester) eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for {
		select {
		case t := <-d.TradingChan:
			json.NewEncoder(w).Encode(t)
		}
	}
}

func (d *Digester) Digest() {
	go func() {
		http.HandleFunc("/events", d.eventsHandler)
		http.ListenAndServe(":8080", nil)
	}()

	for {
		select {
		case m := <-d.MessageChan:
			log.Debug().Str("account_id", m.AccountId).Str("tx_hash", m.Hash).Uint64("lt", m.Lt).Msg("New transaction")
			go func(m *Message) {
				d.MakeTrading(m)
			}(m)
		}
	}
}
