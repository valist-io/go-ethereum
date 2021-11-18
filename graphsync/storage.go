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

	id := cidFromString(key)
	nh := cidToKeccak256(id)

	return s.db.Node(nh)
}

// cidToKeccak256 returns the keccak hash from the given CID.
func cidToKeccak256(id cid.Cid) common.Hash {
	dec, err := multihash.Decode(id.Hash())
	if err != nil {
		panic(err)
	}
	return common.BytesToHash(dec.Digest)
}

// cidFromString converts the given CID key string to a CID.
func cidFromString(key string) cid.Cid {
	_, id, err := cid.CidFromBytes([]byte(key))
	if err != nil {
		panic(err)
	}
	return id
}
