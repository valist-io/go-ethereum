package portal

import (
	"context"
	"time"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

const ReprovideInterval = time.Hour * 12

type backend interface {
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	ChainDb() ethdb.Database
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
}

type service struct {
	ctx    context.Context
	eth    backend
	state  state.Database
	host   host.Host
	prov   provider.System
	cancel context.CancelFunc
	dstore datastore.Datastore
}

func New(stack *node.Node, eth backend) error {
	ctx, cancel := context.WithCancel(context.Background())

	// node key does not work because curves are incompatible
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return err
	}

	host, router, err := NewHost(ctx, priv)
	if err != nil {
		return err
	}

	if err := Bootstrap(ctx, host); err != nil {
		return err
	}

	dspath := stack.Config().ResolvePath("portaldata")
	dstore, err := leveldb.NewDatastore(dspath, nil)
	if err != nil {
		return err
	}

	queue, err := queue.NewQueue(ctx, "reprovider", dstore)
	if err != nil {
		return err
	}

	bstore := NewBlockstore(eth.ChainDb())
	bsprov := simple.NewBlockstoreProvider(bstore)
	reprov := simple.NewReprovider(ctx, ReprovideInterval, router, bsprov)
	prov := simple.NewProvider(ctx, queue, router)

	stack.RegisterLifecycle(&service{
		ctx:    ctx,
		eth:    eth,
		host:   host,
		cancel: cancel,
		dstore: dstore,
		prov:   provider.NewSystem(prov, reprov),
		state:  state.NewDatabase(eth.ChainDb()),
	})

	return nil
}

func (svc *service) Start() error {
	log.Info("Starting Portal Network", "peer_id", svc.host.ID().Pretty())
	svc.prov.Run()
	go svc.provideLoop()
	return nil
}

func (svc *service) Stop() error {
	log.Info("Stopping Portal Network")
	svc.cancel()
	svc.prov.Close()
	svc.dstore.Close()
	return nil
}

// provideLoop listens for chain head events and provides new state to the portal network.
func (svc *service) provideLoop() {
	headEventCh := make(chan core.ChainHeadEvent)
	headEventSub := svc.eth.SubscribeChainHeadEvent(headEventCh)
	defer headEventSub.Unsubscribe()

	for {
		select {
		case headEvent := <-headEventCh:
			if err := svc.provideState(headEvent.Block); err != nil {
				log.Warn("Error providing state", "error", err)
			}
		case err := <-headEventSub.Err():
			log.Warn("Error from chain head event subscription", "error", err)
			return
		case <-svc.ctx.Done():
			return
		}
	}
}

// provideState provides new state nodes to the portal network.
func (svc *service) provideState(block *types.Block) error {
	parent, err := svc.eth.BlockByHash(svc.ctx, block.ParentHash())
	if err != nil {
		return err
	}

	oldTrie, err := svc.state.OpenTrie(parent.Root())
	if err != nil {
		return err
	}

	newTrie, err := svc.state.OpenTrie(block.Root())
	if err != nil {
		return err
	}

	// iterate all differences between the new state and parent state
	it, _ := trie.NewDifferenceIterator(oldTrie.NodeIterator(nil), newTrie.NodeIterator(nil))
	for it.Next(true) {
		// attempt to provide new state CID
		id := Keccak256ToCid(cid.EthStateTrie, it.Hash())
		if err := svc.prov.Provide(id); err != nil {
			log.Warn("Failed to provide state", "error", err)
		} else {
			log.Info("Provided state", "CID", id.String())
		}

		// if leaf node traverse storage
		if !it.Leaf() {
			continue
		}

		if err := svc.provideStorage(it.LeafBlob()); err != nil {
			log.Warn("Failed to provide storage", "error", err)
		}
	}

	return it.Error()
}

// provideStorage provides new storage nodes to the portal network.
func (svc *service) provideStorage(blob []byte) error {
	var acc types.StateAccount
	if err := rlp.DecodeBytes(blob, &acc); err != nil {
		return err
	}

	storageTrie, err := svc.state.OpenTrie(acc.Root)
	if err != nil {
		return err
	}

	it := storageTrie.NodeIterator(nil)
	for it.Next(true) {
		// attempt to provide new storage CID
		id := Keccak256ToCid(cid.EthStateTrie, it.Hash())
		if err := svc.prov.Provide(id); err != nil {
			log.Warn("Failed to provide storage", "error", err)
		} else {
			log.Info("Provided storage", "CID", id.String())
		}
	}

	return it.Error()
}
