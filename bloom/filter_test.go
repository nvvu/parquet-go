package bloom_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/segmentio/parquet-go/bloom"
)

func TestSplitBlockFilter(t *testing.T) {
	const N = 1000
	const S = 3
	f := make(bloom.SplitBlockFilter, bloom.NumSplitBlocksOf(N, 10))
	p := rand.New(rand.NewSource(S))

	// Half of the values are inserted individually.
	for i := 0; i < N/2; i++ {
		f.Insert(p.Uint64())
	}
	// The other half is inserted as a bulk operation.
	b := make([]uint64, N/2)
	for i := range b {
		b[i] = p.Uint64()
	}
	f.InsertBulk(b)

	if f.Block(0) == nil {
		t.Fatal("looking up filter block returned impossible nil value")
	}

	for _, test := range []struct {
		scenario string
		filter   bloom.Filter
	}{
		{scenario: "filter", filter: f},
		{scenario: "reader", filter: newSerializedFilter(f.Bytes())},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			p.Seed(S)
			falsePositives := 0

			for i := 0; i < N; i++ {
				x := p.Uint64()

				if !test.filter.Check(x) {
					t.Fatalf("bloom filter block does not contain the value #%d that was inserted: %d", i, x)
				}
				if test.filter.Check(^x) {
					falsePositives++
				}
			}

			if r := (float64(falsePositives) / N); r > 0.01 {
				t.Fatalf("bloom filter triggered too many false positives: %g%%", r*100)
			}
		})
	}

	t.Run("Reset", func(t *testing.T) {
		allZeros := true
		for _, b := range f.Bytes() {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			t.Fatal("bloom filter bytes were all zero after inserting keys")
		}
		f.Reset()
		for i, b := range f.Bytes() {
			if b != 0 {
				t.Fatalf("bloom filter byte at index %d was not zero after resetting the filter: %02X", i, b)
			}
		}
	})
}

type serializedFilter struct {
	bytes.Reader
	buf bloom.Block
}

func (f *serializedFilter) Check(x uint64) bool {
	ok, _ := bloom.CheckSplitBlock(&f.Reader, f.Size(), &f.buf, x)
	return ok
}

func newSerializedFilter(b []byte) *serializedFilter {
	f := new(serializedFilter)
	f.Reset(b)
	return f
}

func BenchmarkFilterInsertBulk(b *testing.B) {
	f := make(bloom.SplitBlockFilter, 99)
	x := make([]uint64, 16)
	r := rand.NewSource(0).(rand.Source64)

	for i := range x {
		x[i] = r.Uint64()
	}

	for i := 0; i < b.N; i++ {
		f.InsertBulk(x)
	}

	b.SetBytes(bloom.BlockSize * int64(len(x)))
}

func BenchmarkFilterInsert(b *testing.B) {
	f := make(bloom.SplitBlockFilter, 1)
	for i := 0; i < b.N; i++ {
		f.Insert(uint64(i))
	}
	b.SetBytes(bloom.BlockSize)
}

func BenchmarkFilterCheck(b *testing.B) {
	f := make(bloom.SplitBlockFilter, 1)
	f.Insert(42)
	for i := 0; i < b.N; i++ {
		f.Check(42)
	}
	b.SetBytes(bloom.BlockSize)
}