package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type LocalCache struct {
	store *cache.Cache
}

func NewLocalCache(defaultExpiration, cleanupInterval time.Duration) *LocalCache {
	return &LocalCache{
		store: cache.New(defaultExpiration, cleanupInterval),
	}
}

func (c *LocalCache) Set(key string, value interface{}, d time.Duration) {
	c.store.Set(key, value, d)
}

func (c *LocalCache) Get(key string) (interface{}, bool) {
	return c.store.Get(key)
}
