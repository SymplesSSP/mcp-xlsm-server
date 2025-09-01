package cache

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"

	"mcp-xlsm-server/internal/models"
)

type SmartCache struct {
	lru        *lru.Cache
	hotData    map[string]*models.HotEntry
	mu         sync.RWMutex
	maxMemory  int64
	currentMem int64
	stats      *CacheStats
}

type CacheStats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	HotPromotions int64
	mu          sync.RWMutex
}

func NewSmartCache(maxMemoryMB int64) (*SmartCache, error) {
	maxMemory := maxMemoryMB * 1024 * 1024 // Convert to bytes
	
	lruCache, err := lru.NewWithEvict(1000, func(key interface{}, value interface{}) {
		// Eviction callback
	})
	if err != nil {
		return nil, err
	}

	cache := &SmartCache{
		lru:        lruCache,
		hotData:    make(map[string]*models.HotEntry),
		maxMemory:  maxMemory,
		currentMem: 0,
		stats:      &CacheStats{},
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache, nil
}

func (c *SmartCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get from LRU cache
	value, found := c.lru.Get(key)
	
	if found {
		c.stats.recordHit()
		c.updateHotData(key, true)
		return value, true
	}

	c.stats.recordMiss()
	return nil, false
}

func (c *SmartCache) Set(key string, value interface{}, size int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check memory limit
	if c.currentMem+size > c.maxMemory {
		if !c.evictColdData(size) {
			// Still not enough space
			return false
		}
	}

	// Remove old value if exists
	if _, exists := c.lru.Get(key); exists {
		if oldEntry, ok := c.hotData[key]; ok {
			c.currentMem -= oldEntry.Size
		}
	}

	// Add new value
	c.lru.Add(key, value)
	c.currentMem += size

	// Update hot data tracking
	c.hotData[key] = &models.HotEntry{
		AccessCount: 1,
		LastAccess:  time.Now(),
		TTL:         5 * time.Minute,
		Size:        size,
	}

	return true
}

func (c *SmartCache) updateHotData(key string, isHit bool) {
	if entry, exists := c.hotData[key]; exists {
		entry.AccessCount++
		entry.LastAccess = time.Now()

		// Promote to hot data if frequently accessed
		if entry.AccessCount > 3 && entry.TTL < 10*time.Minute {
			entry.TTL = 10 * time.Minute
			c.stats.recordHotPromotion()
		}
	} else if isHit {
		// This shouldn't happen, but handle gracefully
		c.hotData[key] = &models.HotEntry{
			AccessCount: 1,
			LastAccess:  time.Now(),
			TTL:         5 * time.Minute,
			Size:        0, // Unknown size
		}
	}
}

func (c *SmartCache) evictColdData(neededSpace int64) bool {
	threshold := time.Now().Add(-5 * time.Minute)
	freedSpace := int64(0)

	// First pass: evict cold data (low access count and old)
	for key, entry := range c.hotData {
		if freedSpace >= neededSpace {
			break
		}

		if entry.AccessCount < 2 && entry.LastAccess.Before(threshold) {
			c.lru.Remove(key)
			c.currentMem -= entry.Size
			freedSpace += entry.Size
			delete(c.hotData, key)
			c.stats.recordEviction()
		}
	}

	// Second pass: evict old data regardless of access count
	if freedSpace < neededSpace {
		olderThreshold := time.Now().Add(-15 * time.Minute)
		
		for key, entry := range c.hotData {
			if freedSpace >= neededSpace {
				break
			}

			if entry.LastAccess.Before(olderThreshold) {
				c.lru.Remove(key)
				c.currentMem -= entry.Size
				freedSpace += entry.Size
				delete(c.hotData, key)
				c.stats.recordEviction()
			}
		}
	}

	return freedSpace >= neededSpace
}

func (c *SmartCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.hotData[key]; exists {
		c.currentMem -= entry.Size
		delete(c.hotData, key)
	}

	c.lru.Remove(key)
}

func (c *SmartCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Purge()
	c.hotData = make(map[string]*models.HotEntry)
	c.currentMem = 0
}

func (c *SmartCache) GetStats() CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	return CacheStats{
		Hits:          c.stats.Hits,
		Misses:        c.stats.Misses,
		Evictions:     c.stats.Evictions,
		HotPromotions: c.stats.HotPromotions,
	}
}

func (c *SmartCache) GetMemoryUsage() (int64, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.currentMem, c.maxMemory
}

func (c *SmartCache) GetHitRatio() float64 {
	stats := c.GetStats()
	total := stats.Hits + stats.Misses
	if total == 0 {
		return 0
	}
	return float64(stats.Hits) / float64(total)
}

func (c *SmartCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *SmartCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	
	for key, entry := range c.hotData {
		// Remove expired entries
		if now.Sub(entry.LastAccess) > entry.TTL {
			c.lru.Remove(key)
			c.currentMem -= entry.Size
			delete(c.hotData, key)
		}
	}
}

// Cache entry with metadata
type CacheEntry struct {
	Value     interface{}
	Size      int64
	CreatedAt time.Time
	ExpiresAt time.Time
	Checksum  string
}

func (c *SmartCache) SetWithMetadata(key string, entry *CacheEntry) bool {
	return c.Set(key, entry, entry.Size)
}

func (c *SmartCache) GetWithValidation(key string, expectedChecksum string) (interface{}, bool) {
	value, found := c.Get(key)
	if !found {
		return nil, false
	}

	if entry, ok := value.(*CacheEntry); ok {
		// Check expiration
		if time.Now().After(entry.ExpiresAt) {
			c.Delete(key)
			return nil, false
		}

		// Check checksum if provided
		if expectedChecksum != "" && entry.Checksum != expectedChecksum {
			c.Delete(key) // Invalidate stale data
			return nil, false
		}

		return entry.Value, true
	}

	return value, true
}

// Statistics methods
func (s *CacheStats) recordHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hits++
}

func (s *CacheStats) recordMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Misses++
}

func (s *CacheStats) recordEviction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Evictions++
}

func (s *CacheStats) recordHotPromotion() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HotPromotions++
}

// Specialized cache for file checksums
type ChecksumCache struct {
	cache map[string]string
	mu    sync.RWMutex
}

func NewChecksumCache() *ChecksumCache {
	return &ChecksumCache{
		cache: make(map[string]string),
	}
}

func (cc *ChecksumCache) Set(filepath, checksum string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.cache[filepath] = checksum
}

func (cc *ChecksumCache) Get(filepath string) (string, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	checksum, exists := cc.cache[filepath]
	return checksum, exists
}

func (cc *ChecksumCache) IsChanged(filepath, newChecksum string) bool {
	oldChecksum, exists := cc.Get(filepath)
	if !exists {
		return true // Treat as changed if not cached
	}
	return oldChecksum != newChecksum
}