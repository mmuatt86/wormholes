// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	ibfttypes "github.com/ethereum/go-ethereum/consensus/istanbul/ibft/types"
	"github.com/ethereum/go-ethereum/core/types"
	elog "github.com/ethereum/go-ethereum/log"
)

func makeBlock(number int64) *types.Block {
	header := &types.Header{
		Difficulty: big.NewInt(0),
		Number:     big.NewInt(number),
		GasLimit:   0,
		GasUsed:    0,
		Time:       0,
	}
	block := &types.Block{}
	return block.WithSeal(header)
}

func newTestProposal() istanbul.Proposal {
	return makeBlock(1)
}

func TestNewRequest(t *testing.T) {
	testLogger.SetHandler(elog.StdoutHandler)

	N := uint64(4)
	F := uint64(1)

	sys := NewTestSystemWithBackend(N, F)

	close := sys.Run(true)
	defer close()

	request1 := makeBlock(1)
	sys.backends[0].NewRequest(request1)

	<-time.After(1 * time.Second)

	request2 := makeBlock(2)
	sys.backends[0].NewRequest(request2)

	<-time.After(1 * time.Second)

	for _, backend := range sys.backends {
		if len(backend.committedMsgs) != 2 {
			t.Fatalf("the number of executed requests mismatch: have %v, want 2", len(backend.committedMsgs))
		}
		if !reflect.DeepEqual(request1.Number(), backend.committedMsgs[0].commitProposal.Number()) {
			t.Errorf("the number of requests mismatch: have %v, want %v", request1.Number(), backend.committedMsgs[0].commitProposal.Number())
		}
		if !reflect.DeepEqual(request2.Number(), backend.committedMsgs[1].commitProposal.Number()) {
			t.Errorf("the number of requests mismatch: have %v, want %v", request2.Number(), backend.committedMsgs[1].commitProposal.Number())
		}
	}
}

func TestQuorumSize(t *testing.T) {
	N := uint64(4)
	F := uint64(1)

	sys := NewTestSystemWithBackend(N, F)
	backend := sys.backends[0]
	c := backend.engine

	valSet := c.valSet
	for i := 1; i <= 1000; i++ {
		if 2*c.QuorumSize() <= (valSet.Size()+valSet.F()) || 2*c.QuorumSize() > (valSet.Size()+valSet.F()+2) {
			t.Errorf("quorumSize constraint failed, expected value (2*QuorumSize > Size+F && 2*QuorumSize <= Size+F+2) to be:%v, got: %v, for size: %v", true, false, valSet.Size())
		}
	}
}

func TestNilCommittedSealWithEmptyProposal(t *testing.T) {
	N := uint64(4)
	F := uint64(1)

	sys := NewTestSystemWithBackend(N, F)
	backend := sys.backends[0]
	c := backend.engine
	// Set the current round state with an empty proposal
	preprepare := &istanbul.Preprepare{
		View: c.currentView(),
	}
	c.current.SetPreprepare(preprepare)

	// Create a Commit message
	subject := &istanbul.Subject{
		View:   c.currentView(),
		Digest: common.HexToHash("1234567890"),
	}
	subjectPayload, err := ibfttypes.Encode(subject)
	if err != nil {
		t.Errorf("problem with encoding: %v", err)
	}
	msg := &ibfttypes.Message{
		Code: ibfttypes.MsgCommit,
		Msg:  subjectPayload,
	}

	c.finalizeMessage(msg)

	if msg.CommittedSeal != nil {
		t.Errorf("Unexpected committed seal: %s", msg.CommittedSeal)
	}
}

func TestOnlineProofWriteAndRead(t *testing.T) {
	testLogger.SetHandler(elog.StdoutHandler)

	N := uint64(4)
	F := uint64(1)

	sys := NewTestSystemWithBackend(N, F)
	backend := sys.backends[0]
	c := backend.engine
	c.Start()
	defer c.Stop()

	onlineValidators := new(types.OnlineValidatorList)
	if c.onlineProofs == nil {
		c.onlineProofs = make(map[uint64]*types.OnlineValidatorList)
	}
	c.onlineProofs[0] = onlineValidators

	// add data
	go func() {
		fmt.Println("excute write")
		fmt.Println("onlineproof----", c.onlineProofs)
		i := 0
		for {
			c.onlineProofsMu.Lock()
			onlineValidators := c.onlineProofs[uint64(i)]
			if onlineValidators != nil {
				validator := types.NewOnlineValidator(c.current.sequence, common.HexToAddress("0x123456"), common.Hash{}, []byte{})
				onlineValidators.Validators = append(onlineValidators.Validators, validator)
				fmt.Println("success add data: i****", i)
			}

			i++
			c.onlineProofsMu.Unlock()
			time.Sleep(300 * time.Millisecond)
		}
	}()

	// delete data
	go func() {
		fmt.Println("excute delete")
		i := 0
		for {
			c.onlineProofsMu.Lock()
			fmt.Println("start delete data:i-----------------", i)
			delete(c.onlineProofs, uint64(i))
			i++
			c.onlineProofsMu.Unlock()
			fmt.Println("end delete data: i------------------", i)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// read data 1
	go func() {
		fmt.Println("excute read")
		for {
			c.GetOnlineProofsMu().Lock()
			fmt.Println("start to read###########")
			onlineValidators := c.onlineProofs
			for k, v := range onlineValidators {
				fmt.Println("---k---", k, "---v---", v.Validators)
			}
			c.GetOnlineProofsMu().Unlock()
			fmt.Println("end to read############")
			time.Sleep(200 * time.Millisecond)
		}
	}()
	time.Sleep(20 * time.Second)
	//select {}
}
