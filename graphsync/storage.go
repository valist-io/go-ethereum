package graphsync

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/trie"
	cid "github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash"

	// register IPLD codecs
	_ "github.com/vulcanize/go-codec-dageth/state_trie"
)

type Storage struct {
	db *trie.Database
}

func (s *Storage) Has(ctx context.Context, key string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	_, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Storage) Get(ctx context.Context, key string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	_, id, err := cid.CidFromBytes([]byte(key))
	if err != nil {
		return nil, err
	}

	dec, err := multihash.Decode(id.Hash())
	if err != nil {
		return nil, err
	}
	
	return s.db.Node(common.BytesToHash(dec.Digest))
}
