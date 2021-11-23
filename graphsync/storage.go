package graphsync

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	cid "github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash"

	// register IPLD codecs
	_ "github.com/vulcanize/go-codec-dageth/header"
	_ "github.com/vulcanize/go-codec-dageth/rct_trie"
	_ "github.com/vulcanize/go-codec-dageth/state_trie"
	_ "github.com/vulcanize/go-codec-dageth/storage_trie"
	_ "github.com/vulcanize/go-codec-dageth/tx_trie"
	_ "github.com/vulcanize/go-codec-dageth/uncles"
)

type Storage struct {
	db ethdb.Database
}

func (s *Storage) Has(ctx context.Context, key string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	_, id, err := cid.CidFromBytes([]byte(key))
	if err != nil {
		return false, err
	}
	dec, err := multihash.Decode(id.Hash())
	if err != nil {
		return false, err
	}

	hash := common.BytesToHash(dec.Digest)
	switch codec := id.Type(); codec {
		case cid.EthBlock:
			number := rawdb.ReadHeaderNumber(s.db, hash)
			if number == nil {
				return false, nil
			}
			return rawdb.HasHeader(s.db, hash, *number), nil
		case cid.EthTxTrie, cid.EthTxReceiptTrie, cid.EthStateTrie, cid.EthStorageTrie:
			return s.db.Has(dec.Digest)
		default:
			return false, fmt.Errorf("invalid multicodec %x", codec)
	}
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

	hash := common.BytesToHash(dec.Digest)
	switch codec := id.Type(); codec {
		case cid.EthBlock:
			number := rawdb.ReadHeaderNumber(s.db, hash)
			if number == nil {
				return nil, fmt.Errorf("header number not found")
			}
			return rawdb.ReadHeaderRLP(s.db, hash, *number), nil
		case cid.EthTxTrie, cid.EthTxReceiptTrie, cid.EthStateTrie, cid.EthStorageTrie:
			return rawdb.ReadTrieNode(s.db, hash), nil
		default:
			return nil, fmt.Errorf("invalid multicodec %x", codec)
	}
}
