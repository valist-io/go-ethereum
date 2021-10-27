package portal

import (
	"context"
	"time"

	datastore "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/libp2p/go-libp2p-core/routing"

	"github.com/ethereum/go-ethereum/ethdb"
)

const ReprovideInterval = time.Hour * 12

func NewProvider(ctx context.Context, router routing.Routing, db ethdb.Database) (provider.System, error) {
	// TODO replace with a persistent datastore
	var dstore datastore.Datastore
	dstore = datastore.NewMapDatastore()
	dstore = sync.MutexWrap(dstore)

	queue, err := queue.NewQueue(ctx, "reprovider", dstore)
	if err != nil {
		return nil, err
	}

	bstore := NewBlockstore(db)
	bsprov := simple.NewBlockstoreProvider(bstore)
	reprov := simple.NewReprovider(ctx, ReprovideInterval, router, bsprov)
	prov := simple.NewProvider(ctx, queue, router)

	return provider.NewSystem(prov, reprov), nil
}
