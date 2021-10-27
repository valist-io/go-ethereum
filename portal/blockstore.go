package portal

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

// Blockstore is a read-only geth backed blockstore.
type Blockstore struct {
	db ethdb.Database
}

// NewBlockstore returns a read-only geth backed blockstore.
func NewBlockstore(db ethdb.Database) *Blockstore {
	return &Blockstore{db}
}

func (b *Blockstore) Has(id cid.Cid) (bool, error) {
	return b.db.Has(CidToKeccak256(id))
}

func (b *Blockstore) Get(id cid.Cid) (blocks.Block, error) {
	data, err := b.db.Get(CidToKeccak256(id))
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, id)
}

func (b *Blockstore) GetSize(id cid.Cid) (int, error) {
	data, err := b.db.Get(CidToKeccak256(id))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

// AllKeysChan returns a channel that contains all trie node keys from the geth database.
func (b *Blockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	it := b.db.NewIterator(nil, nil)
	kc := make(chan cid.Cid)

	go func() {
		defer func() {
			it.Release()
			close(kc)
		}()

		for it.Next() {
			var id cid.Cid
			key := it.Key()

			switch {
			case len(key) == common.HashLength:
				id = Keccak256ToCid(cid.EthStateTrie, common.BytesToHash(key))
			default:
				continue
			}

			select {
			case kc <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	return kc, nil
}

func (*Blockstore) HashOnRead(bool)              {}
func (*Blockstore) DeleteBlock(cid.Cid) error    { return fmt.Errorf("read-only blockstore") }
func (*Blockstore) Put(blocks.Block) error       { return fmt.Errorf("read-only blockstore") }
func (*Blockstore) PutMany([]blocks.Block) error { return fmt.Errorf("read-only blockstore") }
