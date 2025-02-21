package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/pkg/crawler"
	"github.com/tdex-network/tdex-daemon/pkg/explorer"
	"github.com/vulpemventures/go-elements/network"
)

//BlockchainListener defines the needed method sto start and stop a blockchain listener
type BlockchainListener interface {
	StartObservation()
	StopObservation()

	StartObserveAddress(accountIndex int, addr string, blindKey []byte)
	StopObserveAddress(addr string)

	StartObserveTx(txid string, market Market)
	StopObserveTx(txid string)

	StartObserveOutpoints(outpoints []explorer.Utxo, tradeID string)
	StopObserveOutpoints(outpoints interface{})

	PubSubService() ports.SecurePubSub
}

type blockchainListener struct {
	crawlerSvc         crawler.Service
	repoManager        ports.RepoManager
	pubsubSvc          ports.SecurePubSub
	started            bool
	pendingObservables []crawler.Observable
	network            *network.Network

	mutex *sync.RWMutex
}

// NewBlockchainListener returns a BlockchainListener with all the needed services
func NewBlockchainListener(
	crawlerSvc crawler.Service,
	repoManager ports.RepoManager,
	pubsubSvc ports.SecurePubSub,
	net *network.Network,
) BlockchainListener {
	return newBlockchainListener(
		crawlerSvc,
		repoManager,
		pubsubSvc,
		net,
	)
}

func newBlockchainListener(
	crawlerSvc crawler.Service,
	repoManager ports.RepoManager,
	pubsubSvc ports.SecurePubSub,
	net *network.Network,
) *blockchainListener {
	return &blockchainListener{
		crawlerSvc:         crawlerSvc,
		repoManager:        repoManager,
		pubsubSvc:          pubsubSvc,
		mutex:              &sync.RWMutex{},
		pendingObservables: make([]crawler.Observable, 0),
		network:            net,
	}
}

func (b *blockchainListener) StartObservation() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.started {
		log.Debug("start crawler")
		go b.crawlerSvc.Start()
		log.Debug("start listening on event channel")
		go b.listenToEventChannel()
		go b.startPendingObservables()
		b.started = true
	}
}

func (b *blockchainListener) StopObservation() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.started {
		log.Debug("stop crawler")
		b.crawlerSvc.Stop()
		b.started = false
	}
}

func (b *blockchainListener) StartObserveAddress(
	accountIndex int,
	addr string,
	blindKey []byte,
) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	observable := crawler.NewAddressObservable(addr, blindKey, accountIndex)

	b.addOrQueueObservable(observable)
}

func (b *blockchainListener) StartObserveTx(txid string, mkt Market) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	extraData := map[string]interface{}{
		"baseasset":  mkt.BaseAsset,
		"quoteasset": mkt.QuoteAsset,
	}
	observable := crawler.NewTransactionObservable(txid, extraData)

	b.addOrQueueObservable(observable)
}

func (b *blockchainListener) StartObserveOutpoints(
	utxos []explorer.Utxo, tradeID string,
) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	outs := make([]crawler.Outpoint, 0, len(utxos))
	for _, u := range utxos {
		outs = append(outs, u)
	}
	extraData := map[string]interface{}{
		"tradeid": tradeID,
	}
	observable := crawler.NewOutpointsObservable(outs, extraData)

	b.addOrQueueObservable(observable)
}

func (b *blockchainListener) StopObserveAddress(addr string) {
	b.crawlerSvc.RemoveObservable(&crawler.AddressObservable{
		Address: addr,
	})
}

func (b *blockchainListener) StopObserveTx(txid string) {
	b.crawlerSvc.RemoveObservable(&crawler.TransactionObservable{TxID: txid})
}

func (b *blockchainListener) StopObserveOutpoints(utxos interface{}) {
	var outs []crawler.Outpoint

	if list, ok := utxos.([]crawler.Outpoint); ok {
		outs = list
	} else {
		list := utxos.([]explorer.Utxo)
		outs = make([]crawler.Outpoint, 0, len(list))
		for _, u := range list {
			outs = append(outs, u)
		}
	}

	b.crawlerSvc.RemoveObservable(&crawler.OutpointsObservable{
		Outpoints: outs,
	})
}

func (b *blockchainListener) PubSubService() ports.SecurePubSub {
	return b.pubsubSvc
}

func (b *blockchainListener) listenToEventChannel() {
	for {
		event := <-b.crawlerSvc.GetEventChannel()

		switch event.Type() {
		default:
			// unnoticeable sleep to prevent high cpu usage
			// https://github.com/golang/go/issues/27707#issuecomment-698487427
			time.Sleep(time.Microsecond)
		case crawler.CloseSignal:
			log.Trace("CloseEvent detected")
			log.Trace("stop listening on event channel")
			return
		case crawler.TransactionConfirmed, crawler.TransactionUnconfirmed:
			e := event.(crawler.TransactionEvent)
			isTxConfirmed := e.Type() == crawler.TransactionConfirmed

			market, err := extractMarket(e.ExtraData)
			if err != nil {
				log.WithError(err).Warn(
					"an error occured while retrieving market from event",
				)
				break
			}

			if err := b.updateUtxoSet(
				e.TxHex, e.TxID, market, isTxConfirmed,
			); err != nil {
				log.Warnf("trying to confirm or add unspents: %v", err)
				break
			}

			// stop watching for a tx after it's confirmed
			if isTxConfirmed {
				b.StopObserveTx(e.TxID)
			}
		case crawler.OutpointsSpentAndUnconfirmed, crawler.OutpointsSpentAndConfirmed:
			e := event.(crawler.OutpointsEvent)

			txIsConfirmed := event.Type() == crawler.OutpointsSpentAndConfirmed

			tradeID, err := extractTradeID(e.ExtraData)
			if err != nil {
				log.WithError(err).Warn(
					"an error occured while retrieving tradeID from event",
				)
				break
			}

			trade, err := b.repoManager.TradeRepository().GetOrCreateTrade(
				context.Background(), tradeID,
			)
			if err != nil {
				log.WithError(err).Warn("an error occured while retrieving trade")
				break
			}

			if trade.Status.Failed {
				b.StopObserveOutpoints(e.Outpoints)
				break
			}

			if txIsConfirmed {
				if err := b.settleTrade(tradeID, e.BlockTime, e.TxHex, e.TxID); err != nil {
					log.WithError(err).Warnf(
						"an error occured while settling trade with id %s", tradeID,
					)
					break
				}
				// Publish message for topic TradeSettled to pubsub service.
				go b.publishTradeSettledEvent(trade)
				// Stop watching outpoints.
				b.StopObserveOutpoints(e.Outpoints)
			}

			mkt := &Market{
				BaseAsset:  trade.MarketBaseAsset,
				QuoteAsset: trade.MarketQuoteAsset,
			}
			if err := b.updateUtxoSet(
				e.TxHex, e.TxID, mkt, txIsConfirmed,
			); err != nil {
				log.WithError(err).Warnf(
					"an error occured while confirming or addding unspents",
				)
			}
		}
	}
}

func (b *blockchainListener) startPendingObservables() {
	if len(b.pendingObservables) <= 0 {
		return
	}

	for _, observable := range b.pendingObservables {
		b.crawlerSvc.AddObservable(observable)
		time.Sleep(200 * time.Millisecond)
	}

	b.pendingObservables = nil
}

func (b *blockchainListener) settleTrade(
	tradeID *uuid.UUID, blockTime int, txHex, txID string,
) error {
	if err := b.repoManager.TradeRepository().UpdateTrade(
		context.Background(),
		tradeID,
		func(t *domain.Trade) (*domain.Trade, error) {
			mustAddTxHex := t.IsAccepted()
			if _, err := t.Settle(uint64(blockTime)); err != nil {
				return nil, err
			}
			if mustAddTxHex {
				t.TxHex = txHex
			}
			if t.TxID == "" {
				t.TxID = txID
			}

			return t, nil
		},
	); err != nil {
		return err
	}

	log.Infof("trade with id %s settled", tradeID)
	return nil
}

func (b *blockchainListener) updateUtxoSet(
	txHex, txID string, mkt *Market, isTxConfirmed bool,
) error {
	ctx := context.Background()
	accountIndex := domain.FeeAccount
	var err error

	if mkt != nil {
		_, accountIndex, err = b.repoManager.MarketRepository().GetMarketByAssets(
			ctx, mkt.BaseAsset, mkt.QuoteAsset,
		)
		if err != nil {
			return err
		}
	}

	unspentsToAddOrConfirm, unspentsToSpend, err := extractUnspentsFromTx(
		b.repoManager.VaultRepository(),
		b.network,
		txHex,
		accountIndex,
	)
	if err != nil {
		return err
	}

	count, err := b.repoManager.UnspentRepository().AddUnspents(
		ctx, unspentsToAddOrConfirm,
	)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Debugf("added %d unspents", count)
	}

	count, err = b.repoManager.UnspentRepository().SpendUnspents(
		ctx, unspentsToSpend,
	)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Debugf("spent %d unspents", count)
	}

	// If the spending tx is in mempool, its inputs can be marked as spent and
	// the outs added to the utxo set. Otherwise, only its outputs need to be
	// marked as confirmed.
	if isTxConfirmed {
		unspentKeys := make([]domain.UnspentKey, 0, len(unspentsToAddOrConfirm))
		for _, u := range unspentsToAddOrConfirm {
			unspentKeys = append(unspentKeys, u.Key())
		}

		count, err := b.repoManager.UnspentRepository().ConfirmUnspents(
			ctx, unspentKeys,
		)
		if err != nil {
			return err
		}
		if count > 0 {
			log.Debugf("confirmed %d unspents", count)
		}
	}

	return nil
}

func (b *blockchainListener) addOrQueueObservable(obs crawler.Observable) {
	if !b.started {
		b.pendingObservables = append(b.pendingObservables, obs)
		return
	}
	b.crawlerSvc.AddObservable(obs)
}

func (b *blockchainListener) publishTradeSettledEvent(trade *domain.Trade) {
	if b.pubsubSvc == nil {
		return
	}

	ctx := context.Background()
	mkt, mktAccount, err := b.repoManager.MarketRepository().GetMarketByAssets(
		ctx, trade.MarketBaseAsset, trade.MarketQuoteAsset,
	)
	if err != nil {
		log.WithError(err).Warn("an error occured while retrieving market")
		return
	}

	info, err := b.repoManager.VaultRepository().GetAllDerivedAddressesInfoForAccount(
		ctx, mktAccount,
	)
	if err != nil {
		log.WithError(err).Warn("an error occured while retrieving market addresses")
		return
	}
	addresses := info.Addresses()
	baseBalance, err := b.repoManager.UnspentRepository().GetBalance(
		ctx, addresses, mkt.BaseAsset,
	)
	if err != nil {
		log.WithError(err).Warn("an error occured while retrieving base balance")
		return
	}
	quoteBalance, err := b.repoManager.UnspentRepository().GetBalance(
		ctx, addresses, mkt.QuoteAsset,
	)
	if err != nil {
		log.WithError(err).Warn("an error occured while retrieving quote balance")
		return
	}

	publishTradeSettledTopic(
		b.pubsubSvc, trade, mkt.BaseAsset, baseBalance, quoteBalance,
	)
}

func extractTradeID(data interface{}) (*uuid.UUID, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("extra data unknown type")
	}
	iTradeID, ok := m["tradeid"]
	if !ok {
		return nil, fmt.Errorf("extra data misses trade ID")
	}
	tradeID, ok := iTradeID.(string)
	if !ok {
		return nil, fmt.Errorf("extra data unknown trade ID type")
	}
	id, err := uuid.Parse(tradeID)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func extractMarket(data interface{}) (*Market, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("extra data unknown type")
	}
	iMktBaseAsset, ok := m["baseasset"]
	if !ok {
		return nil, fmt.Errorf("extra data misses market base asset")
	}
	mktBaseAsset, ok := iMktBaseAsset.(string)
	if !ok {
		return nil, fmt.Errorf("extra data unknown market base asset type")
	}
	iMktQuoteAsset, ok := m["quoteasset"]
	if !ok {
		return nil, fmt.Errorf("extra data misses market quote asset")
	}
	mktQuoteAsset, ok := iMktQuoteAsset.(string)
	if !ok {
		return nil, fmt.Errorf("extra data unknown market quote asset type")
	}

	if len(mktQuoteAsset) == 0 && len(mktBaseAsset) == 0 {
		return nil, nil
	}
	return &Market{
		BaseAsset:  mktBaseAsset,
		QuoteAsset: mktQuoteAsset,
	}, nil
}
