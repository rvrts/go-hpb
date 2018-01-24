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
func newHistorysnap(config *PrometheusConfig, sigcache *lru.ARCCache, number uint64, hash common.Hash, signers []common.Address) *Snapshot {
	snap := &Snapshot{
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

