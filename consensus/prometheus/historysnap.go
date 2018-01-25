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
	"encoding/json"

	"github.com/hpb-project/go-hpb/common"
	"github.com/hpb-project/go-hpb/core/types"
	"github.com/hpb-project/go-hpb/ethdb"
	"github.com/hpb-project/go-hpb/params"
	"github.com/hashicorp/golang-lru"
)

type PrometheusConfig struct {
	Period uint64 `json:"period"` // 打包区块间隔期
	Epoch  uint64 `json:"epoch"`  // 充值投票点时间
}

type Vote struct {
	Signer    common.Address `json:"signer"`    // 可以投票的Signer
	Block     uint64         `json:"block"`     // 开始计票的区块
	Address   common.Address `json:"address"`   // 操作的账户
	Authorize bool           `json:"authorize"` // 投票的建议
}

type Tally struct {
	Authorize bool `json:"authorize"` // 投票的想法，加入还是剔除
	Votes     int  `json:"votes"`     // 通过投票的个数
}

type Historysnap struct {
	config   *PrometheusConfig 
	sigcache *lru.ARCCache       
	Number  uint64                      `json:"number"`  // 生成快照的时间点
	Hash    common.Hash                 `json:"hash"`    // 生成快照的Block hash
	Signers map[common.Address]struct{} `json:"signers"` // 当前的授权用户
	Recents map[uint64]common.Address   `json:"recents"` // 最近签名者 spam
	Votes   []*Vote                     `json:"votes"`   // 最近的投票
	Tally   map[common.Address]Tally    `json:"tally"`   // 目前的计票情况
}

// 为创世块使用
func newHistorysnap(config *PrometheusConfig, sigcache *lru.ARCCache, number uint64, hash common.Hash, signers []common.Address) *Historysnap {
	snap := &Historysnap{
		config:   config,
		sigcache: sigcache,
		Number:   number,
		Hash:     hash,
		Signers:  make(map[common.Address]struct{}),
		Recents:  make(map[uint64]common.Address),
		Tally:    make(map[common.Address]Tally),
	}
	for _, signer := range signers {
		snap.Signers[signer] = struct{}{}
	}
	return snap
}

func loadHistorysnap(config *PrometheusConfig, sigcache *lru.ARCCache, db ethdb.Database, hash common.Hash) (*Historysnap, error) {
	blob, err := db.Get(append([]byte("clique-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(Historysnap)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache

	return snap, nil
}

// store inserts the snapshot into the database.
func (s *Historysnap) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("clique-"), s.Hash[:]...), blob)
}

// 深度拷贝
func (s *Historysnap) copy() *Historysnap {
	cpy := &Historysnap{
		config:   s.config,
		sigcache: s.sigcache,
		Number:   s.Number,
		Hash:     s.Hash,
		Signers:  make(map[common.Address]struct{}),
		Recents:  make(map[uint64]common.Address),
		Votes:    make([]*Vote, len(s.Votes)),
		Tally:    make(map[common.Address]Tally),
	}
	for signer := range s.Signers {
		cpy.Signers[signer] = struct{}{}
	}
	for block, signer := range s.Recents {
		cpy.Recents[block] = signer
	}
	for address, tally := range s.Tally {
		cpy.Tally[address] = tally
	}
	copy(cpy.Votes, s.Votes)

	return cpy
}

// 判断投票的有效性
func (s *Historysnap) validVote(address common.Address, authorize bool) bool {
	_, signer := s.Signers[address]
	//如果已经在，应该删除，如果不在申请添加才合法
	return (signer && !authorize) || (!signer && authorize)
}

// 投票池中添加
func (s *Snapshot) cast(address common.Address, authorize bool) bool {

	if !s.validVote(address, authorize) {
		return false
	}
	
	if old, ok := s.Tally[address]; ok {
		old.Votes++
		s.Tally[address] = old
	} else {
		s.Tally[address] = Tally{Authorize: authorize, Votes: 1}
	}
	return true
}

// 从投票池中删除
func (s *Snapshot) uncast(address common.Address, authorize bool) bool {

	tally, ok := s.Tally[address]
	if !ok {
		return false
	}

	if tally.Authorize != authorize {
		return false
	}

	if tally.Votes > 1 {
		tally.Votes--
		s.Tally[address] = tally
	} else {
		delete(s.Tally, address)
	}
	return true
}

