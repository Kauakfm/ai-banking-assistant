package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type Cache struct {
	store      *gocache.Cache
	defaultTTL time.Duration
}

func New(defaultTTL, cleanupInterval time.Duration) *Cache {
	return &Cache{
		store:      gocache.New(defaultTTL, cleanupInterval),
		defaultTTL: defaultTTL,
	}
}

func (c *Cache) Get(key string) (any, bool) {
	return c.store.Get(key)
}

func (c *Cache) Set(key string, value any) {
	c.store.Set(key, value, c.defaultTTL)
}

func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

func (c *Cache) Delete(key string) {
	c.store.Delete(key)
}
