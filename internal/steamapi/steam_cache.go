package steamapi

import (
	"sync"
	"time"
)

type cacheEntry struct {
	createdAt time.Time
	val       []byte
}

type SteamCache struct {
	cacheMap map[string]cacheEntry
	mu       *sync.Mutex
}

func NewCache(interval time.Duration) SteamCache {
	cache := SteamCache{
		cacheMap: make(map[string]cacheEntry),
		mu:       &sync.Mutex{},
	}
	go cache.ReapLoop(interval)

	return cache
}

func (cache *SteamCache) Add(key string, val []byte) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.cacheMap[key] = cacheEntry{
		createdAt: time.Now().UTC(),
		val:       val,
	}
}

func (cache *SteamCache) Get(key string) ([]byte, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	val, exists := cache.cacheMap[key]
	return val.val, exists
}

func (cache *SteamCache) ReapLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		cache.reap(time.Now().UTC(), interval)
	}
}

func (cache *SteamCache) reap(now time.Time, last time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for key, val := range cache.cacheMap {
		if val.createdAt.Before(now.Add(-last)) {
			delete(cache.cacheMap, key)
		}
	}
}
