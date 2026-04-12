package infra

import (
	"context"
	"time"

	"github.com/coocood/freecache"
)

type Cache struct {
	cache *freecache.Cache
}

func NewCache(sizeMB int) *Cache {
	return &Cache{
		cache: freecache.NewCache(sizeMB * 1024 * 1024),
	}
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.cache.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.cache.Set([]byte(key), value, int(ttl.Seconds()))
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	c.cache.Del([]byte(key))
	return nil
}

func (c *Cache) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	err := c.cache.Set([]byte(key), value, int(ttl.Seconds()))
	if err == freecache.ErrLargeKey || err == freecache.ErrLargeEntry {
		return false, err
	}
	return err == nil, nil
}

func (c *Cache) Close() error {
	c.cache.Clear()
	return nil
}
