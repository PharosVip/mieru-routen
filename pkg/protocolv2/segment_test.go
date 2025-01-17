// Copyright (C) 2023  mieru authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package protocolv2

import (
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/enfein/mieru/pkg/netutil"
	"github.com/enfein/mieru/pkg/rng"
)

func TestMaxFragmentSize(t *testing.T) {
	testcases := []struct {
		mtu       int
		ipVersion netutil.IPVersion
		transport netutil.TransportProtocol
		want      int
	}{
		{
			1500,
			netutil.IPVersion6,
			netutil.TCPTransport,
			MaxPDU,
		},
		{
			1500,
			netutil.IPVersion4,
			netutil.UDPTransport,
			1472 - udpOverhead,
		},
		{
			1500,
			netutil.IPVersionUnknown,
			netutil.UnknownTransport,
			1440 - udpOverhead,
		},
	}
	for _, tc := range testcases {
		got := MaxFragmentSize(tc.mtu, tc.ipVersion, tc.transport)
		if got != tc.want {
			t.Errorf("MaxFragmentSize() = %d, want %d", got, tc.want)
		}
	}
}

func TestSegmentLessFunc(t *testing.T) {
	seg1 := &segment{
		metadata: &dataAckStruct{
			seq: 1,
		},
	}
	seg2 := &segment{
		metadata: &dataAckStruct{
			seq: 2,
		},
	}
	if !segmentLessFunc(seg1, seg2) {
		t.Errorf("segmentLessFunc(1, 2) = %v, want %v", false, true)
	}
}

func TestSegmentTree(t *testing.T) {
	seg := &segment{
		metadata: &dataAckStruct{
			seq: 100,
		},
		payload: []byte{0},
	}
	st := newSegmentTree(1)

	if err := st.Insert(seg); err != nil {
		t.Fatalf("ReplaceOrInsert() failed: %v", err)
	}
	if err := st.Insert(seg); err == nil {
		t.Fatalf("ReplaceOrInsert() is not failing when tree is full")
	}
	minSeq, err := st.MinSeq()
	if err != nil {
		t.Fatalf("MinSeq() failed: %v", err)
	}
	if minSeq != 100 {
		t.Fatalf("MinSeq() = %d, want %d", minSeq, 100)
	}
	maxSeq, err := st.MaxSeq()
	if err != nil {
		t.Fatalf("MaxSeq() failed: %v", err)
	}
	if maxSeq != 100 {
		t.Fatalf("MaxSeq() = %d, want %d", maxSeq, 100)
	}
	if st.Len() != 1 {
		t.Fatalf("Len() = %d, want %d", st.Len(), 1)
	}
	if st.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want %d", st.Remaining(), 0)
	}

	seg2, err := st.DeleteMin()
	if err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if !reflect.DeepEqual(seg, seg2) {
		t.Fatalf("Segment not equal:\n%v\n%v", seg, seg2)
	}
	if _, err := st.MinSeq(); err == nil {
		t.Fatalf("MinSeq() is not failing when tree is empty")
	}
	if _, err := st.MaxSeq(); err == nil {
		t.Fatalf("MaxSeq() is not failing when tree is empty")
	}
	if st.Len() != 0 {
		t.Fatalf("Len() = %d, want %d", st.Len(), 0)
	}
	if st.Remaining() != 1 {
		t.Fatalf("Remaining() = %d, want %d", st.Remaining(), 1)
	}
}

func TestSegmentTreeBlocking(t *testing.T) {
	rng.InitSeed()
	st := newSegmentTree(1)
	var wg sync.WaitGroup
	wg.Add(2)

	// Producer.
	go func() {
		var i uint32 = 0
		for ; i < 100; i++ {
			seg := &segment{
				metadata: &dataAckStruct{
					seq: i,
				},
				payload: []byte{0},
			}
			s := rand.Intn(10)
			time.Sleep(time.Duration(s) * time.Millisecond)
			st.InsertBlocking(seg)
		}
		wg.Done()
	}()

	// Consumer.
	go func() {
		var i uint32 = 0
		for ; i < 100; i++ {
			s := rand.Intn(10)
			time.Sleep(time.Duration(s) * time.Millisecond)
			seg := st.DeleteMinBlocking()
			seq, err := seg.Seq()
			if err != nil {
				t.Errorf("Seq() failed: %v", err)
				wg.Done()
				return
			}
			if seq != i {
				t.Errorf("sequence number = %d, want %d", seq, i)
				wg.Done()
				return
			}
		}
		wg.Done()
	}()

	wg.Wait()
}
