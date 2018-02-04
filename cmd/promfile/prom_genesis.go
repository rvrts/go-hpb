// Copyright 2017 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://wwp.gnu.org/licenses/>.

package main

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"time"

	"github.com/hpb-project/go-hpb/common"
	"github.com/hpb-project/go-hpb/core"
	"github.com/hpb-project/go-hpb/log"
	"github.com/hpb-project/go-hpb/params"
)


// 基于用户的输入产生genesis
func (p *prometh) makeGenesis() {
	// Construct a default genesis block
	genesis := &core.Genesis{
		Timestamp:  uint64(time.Now().Unix()),
		GasLimit:   4700000,
		Difficulty: big.NewInt(1048576),
		Alloc:      make(core.GenesisAlloc),
		Config: &params.ChainConfig{
			HomesteadBlock: big.NewInt(1),
			EIP150Block:    big.NewInt(2),
			EIP155Block:    big.NewInt(3),
			EIP158Block:    big.NewInt(3),
			ByzantiumBlock: big.NewInt(4),
		},
	}
	// Figure out which consensus engine to choose
	fmt.Println()
	fmt.Println("Wlecome to HPB consensus engine file maker")

	//choice := p.read()
	
	// In the case of clique, configure the consensus parameters
	genesis.Difficulty = big.NewInt(1)
	genesis.Config.Clique = &params.CliqueConfig{
		Period: 15,
		Epoch:  30000,
	}
	fmt.Println()
	fmt.Println("How many seconds should blocks take? (default = 15)")
	genesis.Config.Clique.Period = uint64(p.readDefaultInt(15))

	// We also need the initial list of signers
	fmt.Println()
	fmt.Println("Which accounts are allowed to seal? (only one)")

	var signers []common.Address
	
	/*for {
		if address := p.readAddress(); address != nil {
			signers = append(signers, *address)
			continue
		}
		if len(signers) > 0 {
			break
		}
	}*/
	
	if address := p.readAddress(); address != nil {
		signers = append(signers, *address)
	}
	
	
	// Sort the signers and embed into the extra-data section
	/*for i := 0; i < len(signers); i++ {
		for j := i + 1; j < len(signers); j++ {
			if bytes.Compare(signers[i][:], signers[j][:]) > 0 {
				signers[i], signers[j] = signers[j], signers[i]
			}
		}
	}*/
	genesis.ExtraData = make([]byte, 32+len(signers)*common.AddressLength+65)
	for i, signer := range signers {
		copy(genesis.ExtraData[32+i*common.AddressLength:], signer[:])
	}
	
	fmt.Println()
	fmt.Println("please input the hash of sealed accounts")
	var signersHash []common.AddressHash
	
	inputHash := p.read(); 
	bighash, _ := new(big.Int).SetString(inputHash, 16)
	hash := common.BigToAddressHash(bighash)
	//signersHash = append(signersHash, hash)
	
	genesis.ExtraHash = make([]byte, 32+len(signersHash)*common.AddressLength+65)
	copy(genesis.ExtraHash[32+common.AddressLength:], hash[:])
	
	fmt.Println("%d",genesis.ExtraHash[:])
	
	// Consensus all set, just ask for initial funds and go
	fmt.Println()
	fmt.Println("Which accounts should be pre-funded? (advisable at least one)")
	for {
		// Read the address of the account to fund
		if address := p.readAddress(); address != nil {
			genesis.Alloc[*address] = core.GenesisAccount{
				Balance: new(big.Int).Lsh(big.NewInt(1), 256-7), // 2^256 / 128 (allow many pre-funds without balance overflows)
			}
			continue
		}
		break
	}
	// Add a batch of precompile balances to avoid them getting deleted
	for i := int64(0); i < 256; i++ {
		genesis.Alloc[common.BigToAddress(big.NewInt(i))] = core.GenesisAccount{Balance: big.NewInt(1)}
	}
	fmt.Println()

	// Query the user for some custom extras
	fmt.Println()
	fmt.Println("Specify your chain/network ID if you want an explicit one (default = random)")
	genesis.Config.ChainId = new(big.Int).SetUint64(uint64(p.readDefaultInt(rand.Intn(65536))))

	fmt.Println()
	fmt.Println("Anything fun to embed into the genesis block? (max 32 bytes)")

	extra := p.read()
	if len(extra) > 32 {
		extra = extra[:32]
	}
	genesis.ExtraData = append([]byte(extra), genesis.ExtraData[len(extra):]...)

	// All done, store the genesis and flush to disk
	p.conf.genesis = genesis
}

// manageGenesis permits the modification of chain configuration parameters in
// a genesis config and the export of the entire genesis spec.
func (p *prometh) manageGenesis() {
	// Figure out whether to modify or export the genesis
	fmt.Println()
	fmt.Println(" 1. Modify existing fork rules")
	fmt.Println(" 2. Export genesis configuration")

	choice := p.read()
	switch {
	case choice == "1":
		// Fork rule updating requested, iterate over each fork
		fmt.Println()
		fmt.Printf("Which block should Homestead come into effect? (default = %v)\n", p.conf.genesis.Config.HomesteadBlock)
		p.conf.genesis.Config.HomesteadBlock = p.readDefaultBigInt(p.conf.genesis.Config.HomesteadBlock)

		fmt.Println()
		fmt.Printf("Which block should EIP150 come into effect? (default = %v)\n", p.conf.genesis.Config.EIP150Block)
		p.conf.genesis.Config.EIP150Block = p.readDefaultBigInt(p.conf.genesis.Config.EIP150Block)

		fmt.Println()
		fmt.Printf("Which block should EIP155 come into effect? (default = %v)\n", p.conf.genesis.Config.EIP155Block)
		p.conf.genesis.Config.EIP155Block = p.readDefaultBigInt(p.conf.genesis.Config.EIP155Block)

		fmt.Println()
		fmt.Printf("Which block should EIP158 come into effect? (default = %v)\n", p.conf.genesis.Config.EIP158Block)
		p.conf.genesis.Config.EIP158Block = p.readDefaultBigInt(p.conf.genesis.Config.EIP158Block)

		fmt.Println()
		fmt.Printf("Which block should Byzantium come into effect? (default = %v)\n", p.conf.genesis.Config.ByzantiumBlock)
		p.conf.genesis.Config.ByzantiumBlock = p.readDefaultBigInt(p.conf.genesis.Config.ByzantiumBlock)

		out, _ := json.MarshalIndent(p.conf.genesis.Config, "", "  ")
		fmt.Printf("Chain configuration updated:\n\n%s\n", out)

	case choice == "2":
		// Save whatever genesis configuration we currently have
		fmt.Println()
		fmt.Printf("Which file to save the genesis into? (default = %s.json)\n", p.network)
		out, _ := json.MarshalIndent(p.conf.genesis, "", "  ")
		if err := ioutil.WriteFile(p.readDefaultString(fmt.Sprintf("%s.json", p.network)), out, 0644); err != nil {
			log.Error("Failed to save genesis file", "err", err)
		}
		log.Info("Exported existing genesis block")

	default:
		log.Error("That's not something I can do")
	}
}
