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
