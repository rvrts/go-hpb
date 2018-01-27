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
	"github.com/hpb-project/go-hpb/common"
	"github.com/hpb-project/go-hpb/consensus"
	"github.com/hpb-project/go-hpb/core/types"
	"github.com/hpb-project/go-hpb/rpc"
)


// openAPI struct
type openAPI struct {
	chain  consensus.ChainReader
	prometheus *Prometheus
}

// 根据区块的头Number获取区块头信息
func (openAPI *openAPI) GetBlockchainHeaderByNumber(number *rpc.BlockNumber) (*types.Header){
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = openAPI.chain.CurrentHeader()
	} else {
		header = openAPI.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	return header
}

// 根据区块的Hash获取区块头信息
func (openAPI *openAPI) GetBlockchainHeaderByHash(hash common.Hash) (*types.Header){
	return openAPI.chain.GetHeaderByHash(hash)
}

// 根据去块号获取历史快照
func (openAPI *openAPI) GetHistorysnapAtNumber(number *rpc.BlockNumber) (*Historysnap, error) {
	var header *types.Header
	header = openAPI.GetBlockchainHeaderByNumber(number)
	if header == nil {
		return nil, errUnknownBlock
	}
	return openAPI.prometheus.snapshot(openAPI.chain, header.Number.Uint64(), header.Hash(), nil)
}

// 根据Hash获取快照
func (openAPI *openAPI) GetHistorysnapAtHash(hash common.Hash) (*Historysnap, error) {
	header := openAPI.GetBlockchainHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return openAPI.prometheus.snapshot(openAPI.chain, header.Number.Uint64(), header.Hash(), nil)
}

// 获取指定区块号码的授权地址
func (openAPI *openAPI) GetSignersAtNumber(number *rpc.BlockNumber) ([]common.Address, error) {
	var header *types.Header
	header = openAPI.GetBlockchainHeaderByNumber(number)
	if header == nil {
		return nil, errUnknownBlock
	}
	
	snap, err := openAPI.prometheus.snapshot(openAPI.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// 获取指定区块Hash的授权地址
func (openAPI *openAPI) GetSignersAtHash(hash common.Hash) ([]common.Address, error) {
	header := openAPI.GetBlockchainHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := openAPI.prometheus.snapshot(openAPI.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// 返回当前投票池的情况
func (openAPI *openAPI) Proposals() map[common.Address]bool {
	openAPI.prometheus.lock.RLock()
	defer openAPI.prometheus.lock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range openAPI.prometheus.proposals {
		proposals[address] = auth
	}
	return proposals
}

// 新增一个新的提案
func (openAPI *openAPI) Propose(address common.Address, auth bool) {
	openAPI.prometheus.lock.Lock()
	defer openAPI.prometheus.lock.Unlock()
	openAPI.prometheus.proposals[address] = auth
}

// 从签名者中删除
func (openAPI *openAPI) Discard(address common.Address) {
	openAPI.prometheus.lock.Lock()
	defer openAPI.prometheus.lock.Unlock()

	delete(openAPI.prometheus.proposals, address)
}
