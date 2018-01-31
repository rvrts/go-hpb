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
	"fmt"
	"github.com/hpb-project/go-hpb/common"
	"github.com/hpb-project/go-hpb/consensus"
	"github.com/hpb-project/go-hpb/core/types"
	"github.com/hpb-project/go-hpb/rpc"
)


type API struct {
	chain  consensus.ChainReader
	prometheus *Prometheus
}

func (api *API) GetHistorysnap(number *rpc.BlockNumber) (*Historysnap, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.prometheus.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

func (api *API) GetHistorysnapAtHash(hash common.Hash) (*Historysnap, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.prometheus.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

func (api *API) GetSigners(number *rpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return the signers from its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.prometheus.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

func (api *API) GetSignersAtHash(hash common.Hash) ([]common.Address, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.prometheus.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

func (api *API) Proposals() map[common.Address]bool {
	api.prometheus.lock.RLock()
	defer api.prometheus.lock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.prometheus.proposals {
		proposals[address] = auth
	}
	return proposals
}

func (api *API) Propose(address common.Address, auth bool) {
	api.prometheus.lock.Lock()
	defer api.prometheus.lock.Unlock()
   
    rand :=  pre_random().String()
    
    //address.Str()
    //fmt.Printf("Hex: %s ", address.Hex())
    //fmt.Printf("sha3: %s ", rand)
    //fmt.Printf("Hex + sha3: %s ", address.Hex() + rand)
    //fmt.Printf("sha3: %s ", api.prometheus.Keccak512([]byte(rand)))
    
    phash :=  api.prometheus.fnv_hash([]byte(address.Hex() + rand))
    fmt.Printf("fnv: %s", phash)
    //设置随机数
    api.prometheus.randomStr = rand
    //设置随机后的Hash
    api.prometheus.signerHash = phash
    // 将Hash 推入到proposalsHash中
    api.prometheus.proposalsHash[phash] = auth
	api.prometheus.proposals[address] = auth
}


func (api *API) Discard(address common.Address) {
	api.prometheus.lock.Lock()
	defer api.prometheus.lock.Unlock()

	delete(api.prometheus.proposals, address)
}
