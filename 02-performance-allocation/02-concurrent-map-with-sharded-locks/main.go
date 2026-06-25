package main

import (
	"hash/maphash"
	"sync"
)

var seed = maphash.MakeSeed()

type shard[K comparable, V any] struct {
	id int
	m  map[K]V

	mu sync.RWMutex
}

type ShardedMap[K comparable, V any] struct {
	shards []*shard[K, V]
}

func NewShardedMap[K comparable, V any](num int) *ShardedMap[K, V] {
	sm := &ShardedMap[K, V]{}

	for i := range num {
		s := &shard[K, V]{
			id: i,
			m:  make(map[K]V),
		}

		sm.shards = append(sm.shards, s)
	}

	return sm
}

func (sm *ShardedMap[K, V]) shardIndex(key K) int {
	h := maphash.Comparable(seed, key)
	return int(h % uint64(len(sm.shards)))
}

func (sm *ShardedMap[K, V]) Get(key K) (V, bool) {
	s := sm.shards[sm.shardIndex(key)]
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.m[key]
	return val, ok
}

func (sm *ShardedMap[K, V]) Set(key K, value V) {
	s := sm.shards[sm.shardIndex(key)]
	s.mu.Lock()
	s.m[key] = value
	s.mu.Unlock()
}

func (sm *ShardedMap[K, V]) Delete(key K) {
	s := sm.shards[sm.shardIndex(key)]
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

func (sm *ShardedMap[K, V]) Keys() []K {
	var keys []K

	for _, s := range sm.shards {
		s.mu.RLock()
		for k := range s.m {
			keys = append(keys, k)
		}
		s.mu.RUnlock()
	}

	return keys
}

func main() {
}
