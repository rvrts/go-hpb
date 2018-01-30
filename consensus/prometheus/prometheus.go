// Copyright 2018 The go-hpb Authors
// This file is part of the go-hpb.
//
// The go-hpb is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-hpb is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-hpb. If not, see <http://www.gnu.org/licenses/>.
package prometheus

import (
	"bytes"
	"errors"
	"math/big"
	"math/rand"
	"sync"
	"time"
	"encoding/hex"
	"hash/fnv"

	"github.com/hpb-project/go-hpb/accounts"
	"github.com/hpb-project/go-hpb/common"
	"github.com/hpb-project/go-hpb/common/hexutil"
	"github.com/hpb-project/go-hpb/consensus"
	"github.com/hpb-project/go-hpb/consensus/misc"
	"github.com/hpb-project/go-hpb/core/state"
	"github.com/hpb-project/go-hpb/core/types"
	"github.com/hpb-project/go-hpb/crypto"
	"github.com/hpb-project/go-hpb/crypto/sha3"
	"github.com/hpb-project/go-hpb/ethdb"
	"github.com/hpb-project/go-hpb/log"
	"github.com/hpb-project/go-hpb/params"
	"github.com/hpb-project/go-hpb/rlp"
	"github.com/hpb-project/go-hpb/rpc"
	lru "github.com/hashicorp/golang-lru"
)

const (
	checkpointInterval = 1024 // 投票间隔
	inmemoryHistorysnaps  = 128  // 内存中的快照个数
	inmemorySignatures = 4096 // 内存中的签名个数

	wiggleTime = 500 * time.Millisecond // 延时单位
)

// Prometheus protocol constants.
var (
	epochLength = uint64(30000) // 充值投票的时的间隔，默认 30000个
	blockPeriod = uint64(15)    // 两个区块之间的默认时间 15 秒

	extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = 65 // Fixed number of extra-data suffix bytes reserved for signer seal

	nonceAuthVote = hexutil.MustDecode("0xffffffffffffffff") // Magic nonce number to vote on adding a new signer
	nonceDropVote = hexutil.MustDecode("0x0000000000000000") // Magic nonce number to vote on removing a signer.

	uncleHash = types.CalcUncleHash(nil) // 

	diffInTurn = big.NewInt(2) // 当轮到的时候难度值设置 2
	diffNoTurn = big.NewInt(1) // 当非轮到的时候难度设置 1 
)


// Keccak512 calculates and returns the Keccak512 hash of the input data.
func (c *Prometheus) Keccak512(data ...[]byte) string {
    d := sha3.NewKeccak512()
    for _, b := range data {
        d.Write(b)
    }
    //return string(d.Sum(nil)[:])
    return hex.EncodeToString(d.Sum(nil));
    
}

// Fowler–Noll–Vo is a non-cryptographic hash function created by Glenn Fowler, Landon Curt Noll, and Kiem-Phong Vo.
//The basis of the FNV hash algorithm was taken from an idea sent as reviewer comments to the 
//IEEE POSIX P1003.2 committee by Glenn Fowler and Phong Vo in 1991. In a subsequent ballot round, 
//Landon Curt Noll improved on their algorithm. In an email message to Landon, 
//they named it the Fowler/Noll/Vo or FNV hash.
// https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function
func (c *Prometheus) fnv_hash(data ...[]byte) string {
    d := fnv.New32()
    for _, b := range data {
        d.Write(b)
    }
    return hex.EncodeToString(d.Sum(nil))
}


// 回掉函数
type SignerFn func(accounts.Account, []byte) ([]byte, error)

// 对区块头部进行签名，最小65Byte
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-65], 
		header.MixDigest,
		header.Nonce,
	})
	hasher.Sum(hash[:0])
	return hash
}



// 实现引擎的Prepare函数
func (c *Prometheus) Prepare(chain consensus.ChainReader, header *types.Header) error {
	
	header.Coinbase = common.Address{}
	header.Nonce = types.BlockNonce{}

	number := header.Number.Uint64()

	// Assemble the voting snapshot to check which votes make sense
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if number%c.config.Epoch != 0 {
		c.lock.RLock()

		// Gather all the proposals that make sense voting on
		addresses := make([]common.Address, 0, len(c.proposals))
		for address, authorize := range c.proposals {
			if snap.validVote(address, authorize) {
				addresses = append(addresses, address)
			}
		}
		// If there's pending proposals, cast a vote on them
		if len(addresses) > 0 {
			header.Coinbase = addresses[rand.Intn(len(addresses))]
			if c.proposals[header.Coinbase] {
				copy(header.Nonce[:], nonceAuthVote)
			} else {
				copy(header.Nonce[:], nonceDropVote)
			}
		}
		c.lock.RUnlock()
	}
	// Set the correct difficulty
	header.Difficulty = diffNoTurn
	if snap.inturn(header.Number.Uint64(), c.signer) {
		header.Difficulty = diffInTurn
	}
	// Ensure the extra data has all it's components
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]

	if number%c.config.Epoch == 0 {
		for _, signer := range snap.signers() {
			header.Extra = append(header.Extra, signer[:]...)
		}
	}
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

	// Mix digest is reserved for now, set to empty
	header.MixDigest = common.Hash{}

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = new(big.Int).Add(parent.Time, new(big.Int).SetUint64(c.config.Period))
	if header.Time.Int64() < time.Now().Unix() {
		header.Time = big.NewInt(time.Now().Unix())
	}
	return nil
}

// 获取快照
func (c *Prometheus) snapshot(chain consensus.ChainReader, number uint64, hash common.Hash, parents []*types.Header) (*Historysnap, error) {

	var (
		headers []*types.Header
		snap    *Historysnap
	)
	for snap == nil {
		// 直接使用内存中的，recents存部分
		if s, ok := c.recents.Get(hash); ok {
			snap = s.(*Historysnap)
			break
		}
		// 如果是检查点的时候
		if number%checkpointInterval == 0 {
			if s, err := loadHistorysnap(c.config, c.signatures, c.db, hash); err == nil {
				log.Trace("Prometheus： Loaded voting snapshot form disk", "number", number, "hash", hash)
				snap = s
				break
			}
		}
		// 首次要创建
		if number == 0 {
			genesis := chain.GetHeaderByNumber(0)
			if err := c.VerifyHeader(chain, genesis, false); err != nil {
				return nil, err
			}
			signers := make([]common.Address, (len(genesis.Extra)-extraVanity-extraSeal)/common.AddressLength)
			for i := 0; i < len(signers); i++ {
				copy(signers[i][:], genesis.Extra[extraVanity+i*common.AddressLength:])
			}
			
			//Signers:  make(map[common.Address]struct{}),
			snap = newHistorysnap(c.config, c.signatures, 0, genesis.Hash(), signers)
			if err := snap.store(c.db); err != nil {
				return nil, err
			}
			log.Trace("Stored genesis voting snapshot to disk")
			break
		}
		// 没有发现快照，开始收集Header 然后往回回溯
		var header *types.Header
		if len(parents) > 0 {
			// 如果有指定的父亲，直接用
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number.Uint64() != number {
				return nil, consensus.ErrUnknownAncestor
			}
			parents = parents[:len(parents)-1]
		} else {
			// No explicit parents (or no more left), reach out to the database
			// 没有指定的父亲
			header = chain.GetHeader(hash, number)
			if header == nil {
				return nil, consensus.ErrUnknownAncestor
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.ParentHash
	}
	// Previous snapshot found, apply any pending headers on top of it
	//
	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	snap, err := snap.apply(headers)
	if err != nil {
		return nil, err
	}
	c.recents.Add(snap.Hash, snap)

	// If we've generated a new checkpoint snapshot, save to disk
	if snap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = snap.store(c.db); err != nil {
			return nil, err
		}
		log.Trace("Stored voting snapshot to disk", "number", snap.Number, "hash", snap.Hash)
	}
	return snap, err
}

