package main

import (
	"fmt"
	"sync"
	"time"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Key       string
	Value     interface{}
	ExpireAt  time.Time
	CreatedAt time.Time
}

// Cache represents an in-memory cache with expiration
type Cache struct {
	items      map[string]*CacheItem
	mutex      sync.RWMutex
	maxItems   int
	defaultTTL time.Duration
}

// NewCache creates a new cache instance
func NewCache(defaultTTL time.Duration, maxItems int) *Cache {
	cache := &Cache{
		items:      make(map[string]*CacheItem),
		maxItems:   maxItems,
		defaultTTL: defaultTTL,
	}

	// Start cleanup routine
	go cache.startCleanupRoutine()
	return cache
}

// Set adds an item to the cache with default TTL
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL adds an item to the cache with specified TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheItem{
		Key:       key,
		Value:     value,
		ExpireAt:  time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	// Enforce size limit if needed
	if c.maxItems > 0 && len(c.items) > c.maxItems {
		c.evictOldest()
	}
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.ExpireAt) {
		return nil, false
	}

	return item.Value, true
}

// GetOrSet gets an item or sets it if not found
func (c *Cache) GetOrSet(key string, valueFunc func() interface{}) interface{} {
	// Try to get from cache first
	if value, found := c.Get(key); found {
		return value
	}

	// Not found, generate value
	value := valueFunc()
	c.Set(key, value)
	return value
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.items, key)
}

// Clear
