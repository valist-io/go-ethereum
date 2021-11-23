package graphsync

import (
	"context"

	graphsync "github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	libp2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

type service struct {
	host     host.Host
	exchange graphsync.GraphExchange
	cancel   context.CancelFunc
}

func Register(stack *node.Node, backend ethapi.Backend) error {
	ctx, cancel := context.WithCancel(context.Background())

	// use the node key as the libp2p key
	nodeKey := stack.Config().NodeKey()
	nodeKeyBytes := crypto.FromECDSA(nodeKey)

	hostKey, err := libp2p_crypto.UnmarshalSecp256k1PrivateKey(nodeKeyBytes)
	if err != nil {
		return err
	}

	host, _, err := NewHost(ctx, hostKey)
	if err != nil {
		return err
	}

	storage := Storage{backend.ChainDb()}
	linkSys := cidlink.DefaultLinkSystem()
	linkSys.SetReadStorage(&storage)
	linkSys.TrustedStorage = true

	network := gsnet.NewFromLibp2pHost(host)
	exchange := gsimpl.New(ctx, network, linkSys)

	// exchange.RegisterBlockSentListener(func(p peer.ID, _ graphsync.RequestData, blockData graphsync.BlockData) {
	// 	log.Info("GraphSync Block Sent", "peer_id", p.Pretty(), "cid", blockData.Link().String())
	// })
	exchange.RegisterIncomingRequestHook(func(p peer.ID, _ graphsync.RequestData, _ graphsync.IncomingRequestHookActions) {
		log.Info("GraphSync Incoming Request", "peer_id", p.Pretty())
	})
	exchange.RegisterNetworkErrorListener(func(p peer.ID, _ graphsync.RequestData, err error) {
		log.Warn("GraphSync Network Error", "error", err.Error())
	})
	exchange.RegisterReceiverNetworkErrorListener(func(p peer.ID, err error) {
		log.Warn("GraphSync Receiver Network Error", "error", err.Error())
	})
	exchange.RegisterCompletedResponseListener(func(p peer.ID, _ graphsync.RequestData, status graphsync.ResponseStatusCode) {
		log.Info("GraphSync Completed Response", "peer_id", p.Pretty(), "status", status)
	})

	stack.RegisterLifecycle(&service{host, exchange, cancel})
	return nil
}

func (svc *service) Start() error {
	log.Info("Started graphsync exchange", "peer_id", svc.host.ID().Pretty())
	return nil
}

func (svc *service) Stop() error {
	log.Info("Stopped graphsync exchange")
	svc.cancel()
	return nil
}
