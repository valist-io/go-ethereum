package portal

import (
	"context"

	cid "github.com/ipfs/go-cid"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

var BootstrapPeers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
}

type backend interface {
	ChainDb() ethdb.Database
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
}

type service struct {
	eth     backend
	state   state.Database
	host    host.Host
	prov    provider.System
	quit    chan bool
}

func New(stack *node.Node, eth backend) error {
	ctx := context.Background()

	// node key does not work because curves are incompatible
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return err
	}

	host, router, err := NewHost(ctx, priv)
	if err != nil {
		return err
	}

	prov, err := NewProvider(ctx, router, eth.ChainDb())
	if err != nil {
		return err
	}

	stack.RegisterLifecycle(&service{
		eth:   eth,
		state: state.NewDatabase(eth.ChainDb()),
		host:  host, 
		prov:  prov,
		quit:  make(chan bool),
	})

	return nil
}

func (svc *service) Start() error {
	log.Info("Starting Portal Network", "peer_id", svc.host.ID().Pretty())
	svc.prov.Run()
	go svc.loop()
	return nil
}

func (svc *service) Stop() error {
	log.Info("Stopping Portal Network")
	svc.prov.Close()
	close(svc.quit)
	return nil
}

// Loop listens for chain head events and provides new state to the portal network.
func (svc *service) loop() {
	headEventCh := make(chan core.ChainHeadEvent)
	headEventSub := svc.eth.SubscribeChainHeadEvent(headEventCh)
	defer headEventSub.Unsubscribe()
	
	for {
		select {
		case headEvent := <-headEventCh:
			if err := svc.provide(headEvent.Block); err != nil {
				log.Warn("Error providing state", "error", err)
			}
		case err := <-headEventSub.Err():
			log.Warn("Error from chain head event subscription", "error", err)
			return
		case <-svc.quit:
			return
		}
	}
}

// provide iterates all state nodes and provides them to the portal network.
func (svc *service) provide(block *types.Block) error {
	stateTrie, err := svc.state.OpenTrie(block.Root())
	if err != nil {
		return err
	}

	stateIter := stateTrie.NodeIterator(nil)
	for stateIter.Next(true) {
		stateCID := Keccak256ToCid(cid.EthStateTrie, stateIter.Hash())
		svc.prov.Provide(stateCID)

		if !stateIter.Leaf() {
			continue
		}

		var acc types.StateAccount
		if err := rlp.DecodeBytes(stateIter.LeafBlob(), &acc); err != nil {
			return err
		}

		storageTrie, err := svc.state.OpenTrie(acc.Root)
		if err != nil {
			return err
		}

		storageIter := storageTrie.NodeIterator(nil)
		for storageIter.Next(true) {
			storageCID := Keccak256ToCid(cid.EthStateTrie, storageIter.Hash())
			svc.prov.Provide(storageCID)
		}

		if err := storageIter.Error(); err != nil {
			log.Warn("Failed to iterate storage", "error", err)
		}
	}

	return stateIter.Error()
}