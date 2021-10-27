package portal

import (
	"github.com/ethereum/go-ethereum/common"
	cid "github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash"
)

// Keccak256ToCid returns a CID consisting of the given hash and codec.
func Keccak256ToCid(codec uint64, hash common.Hash) cid.Cid {
	enc, err := multihash.Encode(hash.Bytes(), multihash.KECCAK_256)
	if err != nil {
		panic(err)
	}

	return cid.NewCidV1(codec, multihash.Multihash(enc))
}

// CidToKeccak256 returns the keccak hash from the given CID.
func CidToKeccak256(id cid.Cid) []byte {
	dec, err := multihash.Decode(id.Hash())
	if err != nil {
		panic(err)
	}

	return dec.Digest
}
