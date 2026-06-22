package orderbook

import (
	"math/rand"
	"testing"
)

func TestNextPow2(t *testing.T) {
	cases := map[uint64]uint64{
		0: 1, 1: 1, 2: 2, 3: 4, 4: 4, 5: 8, 7: 8, 8: 8, 9: 16, 1000: 1024, 1024: 1024, 1025: 2048,
	}
	for in, want := range cases {
		if got := nextPow2(in); got != want {
			t.Fatalf("nextPow2(%d) = %d, want %d", in, got, want)
		}
	}
}

// TestOrderIndexAgainstOracle fuzzes the orderIndex against a builtin map oracle
// over random uint64 keys with insert/get/remove churn that forces growth and
// exercises the free-list reuse path.
func TestOrderIndexAgainstOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	for trial := 0; trial < 20; trial++ {
		oi := newOrderIndex(4)
		oracle := make(map[uint64]*Order)

		for op := 0; op < 20000; op++ {
			// Bias key space small enough to force collisions and churn,
			// but include occasional huge keys to stress the hash spread.
			var key uint64
			switch rng.Intn(4) {
			case 0:
				key = uint64(rng.Intn(64))
			case 1:
				key = uint64(rng.Intn(4096))
			default:
				key = rng.Uint64()
			}

			switch rng.Intn(3) {
			case 0: // put
				o := &Order{ID: key}
				oi.put(key, o)
				oracle[key] = o
			case 1: // get
				gotVal, gotOK := oi.get(key)
				wantVal, wantOK := oracle[key]
				if gotOK != wantOK || gotVal != wantVal {
					t.Fatalf("get(%d) = (%v,%v), want (%v,%v)", key, gotVal, gotOK, wantVal, wantOK)
				}
			case 2: // remove
				gotVal, gotOK := oi.remove(key)
				wantVal, wantOK := oracle[key]
				if gotOK != wantOK || gotVal != wantVal {
					t.Fatalf("remove(%d) = (%v,%v), want (%v,%v)", key, gotVal, gotOK, wantVal, wantOK)
				}
				delete(oracle, key)
			}

			if oi.count != len(oracle) {
				t.Fatalf("count mismatch: oi.count=%d oracle=%d", oi.count, len(oracle))
			}
		}

		// Full sweep: every oracle key must be present with matching value.
		for k, v := range oracle {
			got, ok := oi.get(k)
			if !ok || got != v {
				t.Fatalf("final get(%d) = (%v,%v), want (%v,true)", k, got, ok, v)
			}
		}
		// Keys absent from the oracle must be absent from the index.
		for probe := 0; probe < 200; probe++ {
			k := uint64(rng.Intn(8192))
			_, want := oracle[k]
			_, got := oi.get(k)
			if got != want {
				t.Fatalf("presence mismatch for %d: oi=%v oracle=%v", k, got, want)
			}
		}
	}
}

// TestOrderIndexFreeListReuse verifies that removed nodes are recycled from the
// free-list on subsequent puts (no unbounded allocation under add/remove churn).
func TestOrderIndexFreeListReuse(t *testing.T) {
	oi := newOrderIndex(4)
	o := &Order{ID: 1}

	oi.put(1, o)
	if oi.free != nil {
		t.Fatalf("free-list should be empty after first put")
	}
	oi.remove(1)
	if oi.free == nil {
		t.Fatalf("free-list should hold the recycled node after remove")
	}
	recycled := oi.free
	oi.put(2, o)
	if oi.free != nil {
		t.Fatalf("free-list should be empty after reusing the recycled node")
	}
	if got, _ := oi.get(2); got != o {
		t.Fatalf("get(2) after reuse returned wrong value")
	}
	// The node reused for key 2 should be the previously freed node.
	if oi.buckets[oi.index(2)] != recycled {
		t.Fatalf("put did not reuse the recycled node from the free-list")
	}
}
