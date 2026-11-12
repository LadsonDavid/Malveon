package extractor

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// Results must come out in index order however long each piece of work takes.
func TestParallelOrderedEmitsInOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	delays := make([]time.Duration, 200)
	for i := range delays {
		delays[i] = time.Duration(rng.Intn(3000)) * time.Microsecond
	}
	var got []int
	parallelOrdered(len(delays), func(i int) int {
		time.Sleep(delays[i])
		return i * 2
	}, func(i int, r int) {
		if r != i*2 {
			t.Fatalf("item %d got %d", i, r)
		}
		got = append(got, i)
	})
	want := make([]int, len(delays))
	for i := range want {
		want[i] = i
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("out of order: %v", got[:20])
	}
}

func TestParallelOrderedHandlesNothingAndOne(t *testing.T) {
	parallelOrdered(0, func(int) int { t.Fatal("no work"); return 0 }, func(int, int) { t.Fatal("no emit") })
	n := 0
	parallelOrdered(1, func(i int) int { return 5 }, func(i, r int) { n += r })
	if n != 5 {
		t.Fatalf("one item: %d", n)
	}
}
