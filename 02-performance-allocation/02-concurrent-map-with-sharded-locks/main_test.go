package main

import (
	"fmt"
	"sync"
	"testing"
)

func BenchmarkMapCpu(b *testing.B) {
	const numGoroutines = 8

	sm := NewShardedMap[int, int](64)

	var wg sync.WaitGroup

	b.ResetTimer()

	for g := 0; g < numGoroutines; g++ {
		start := g * b.N / numGoroutines
		end := (g + 1) * b.N / numGoroutines

		wg.Add(1)

		go func(start, end int) {
			defer wg.Done()

			for i := start; i < end; i++ {
				sm.Set(i, 10)
			}
		}(start, end)
	}

	wg.Wait()
}

func BenchmarkMapRaceCondition(b *testing.B) {
	const numGoroutines = 8
	sizes := []int{1, 16, 64, 128, 256, 512}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			sm := NewShardedMap[int, int](size)

			for i := range 10000 {
				sm.Set(i, i)
			}

			b.ResetTimer()

			var wg sync.WaitGroup

			for g := range numGoroutines {
				wg.Go(func() {
					for i := 0; i < b.N; i++ {
						key := (i*numGoroutines + g) % 1000
						if i%5 == 0 {
							sm.Set(key, i)
						} else {
							sm.Get(key)
						}
					}
				})
			}

			wg.Wait()
		})
	}
}
