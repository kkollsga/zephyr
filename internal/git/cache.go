package git

import (
	"sync"
	"sync/atomic"
	"time"
)

const defaultTTL = 5 * time.Second

// Cache provides thread-safe caching of git status and diff data.
// When data is stale, it returns the old data immediately and
// refreshes in the background to avoid blocking the UI thread.
type Cache struct {
	mu   sync.RWMutex
	repo *Repo

	status     []FileStatus
	statusTime time.Time

	diffs     map[string]*FileDiff
	diffTimes map[string]time.Time

	stat     map[string][2]int
	statTime time.Time

	ttl time.Duration

	// Background refresh tracking — prevents duplicate goroutines
	statusRefreshing atomic.Bool
	statRefreshing   atomic.Bool
	diffRefreshing   sync.Map // path -> bool
}

// NewCache creates a new cache for the given repository.
func NewCache(repo *Repo) *Cache {
	return &Cache{
		repo:      repo,
		diffs:     make(map[string]*FileDiff),
		diffTimes: make(map[string]time.Time),
		ttl:       defaultTTL,
	}
}

// Status returns cached file statuses. If stale, returns old data
// and triggers a background refresh.
func (c *Cache) Status() ([]FileStatus, error) {
	c.mu.RLock()
	result := c.status
	stale := c.status == nil || time.Since(c.statusTime) >= c.ttl
	c.mu.RUnlock()

	if stale {
		// First call ever — must block to get initial data
		if result == nil {
			return c.statusSync()
		}
		// Subsequent stale calls — return old data, refresh async
		c.refreshStatusAsync()
	}
	return result, nil
}

func (c *Cache) statusSync() ([]FileStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status != nil && time.Since(c.statusTime) < c.ttl {
		return c.status, nil
	}
	status, err := c.repo.Status()
	if err != nil {
		return nil, err
	}
	c.status = status
	c.statusTime = time.Now()
	return status, nil
}

func (c *Cache) refreshStatusAsync() {
	if !c.statusRefreshing.CompareAndSwap(false, true) {
		return // already refreshing
	}
	go func() {
		defer c.statusRefreshing.Store(false)
		status, err := c.repo.Status()
		if err != nil {
			return
		}
		c.mu.Lock()
		c.status = status
		c.statusTime = time.Now()
		c.mu.Unlock()
	}()
}

// FileDiff returns the cached diff for a file. If stale, returns
// old data and triggers a background refresh.
func (c *Cache) FileDiff(path string) (*FileDiff, error) {
	c.mu.RLock()
	result := c.diffs[path]
	t := c.diffTimes[path]
	stale := result == nil || time.Since(t) >= c.ttl
	c.mu.RUnlock()

	if stale {
		if result == nil {
			return c.fileDiffSync(path)
		}
		c.refreshFileDiffAsync(path)
	}
	return result, nil
}

func (c *Cache) fileDiffSync(path string) (*FileDiff, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.diffTimes[path]; ok && time.Since(t) < c.ttl {
		return c.diffs[path], nil
	}
	diff, err := c.repo.DiffFile("HEAD", path)
	if err != nil {
		return nil, err
	}
	c.diffs[path] = diff
	c.diffTimes[path] = time.Now()
	return diff, nil
}

func (c *Cache) refreshFileDiffAsync(path string) {
	if _, loaded := c.diffRefreshing.LoadOrStore(path, true); loaded {
		return // already refreshing this path
	}
	go func() {
		defer c.diffRefreshing.Delete(path)
		diff, err := c.repo.DiffFile("HEAD", path)
		if err != nil {
			return
		}
		c.mu.Lock()
		c.diffs[path] = diff
		c.diffTimes[path] = time.Now()
		c.mu.Unlock()
	}()
}

// DiffStat returns cached per-file +/- counts. If stale, returns
// old data and triggers a background refresh.
func (c *Cache) DiffStat() (map[string][2]int, error) {
	c.mu.RLock()
	result := c.stat
	stale := c.stat == nil || time.Since(c.statTime) >= c.ttl
	c.mu.RUnlock()

	if stale {
		if result == nil {
			return c.diffStatSync()
		}
		c.refreshDiffStatAsync()
	}
	return result, nil
}

func (c *Cache) diffStatSync() (map[string][2]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stat != nil && time.Since(c.statTime) < c.ttl {
		return c.stat, nil
	}
	stat, err := c.repo.DiffStat("HEAD")
	if err != nil {
		return nil, err
	}
	c.stat = stat
	c.statTime = time.Now()
	return stat, nil
}

func (c *Cache) refreshDiffStatAsync() {
	if !c.statRefreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer c.statRefreshing.Store(false)
		stat, err := c.repo.DiffStat("HEAD")
		if err != nil {
			return
		}
		c.mu.Lock()
		c.stat = stat
		c.statTime = time.Now()
		c.mu.Unlock()
	}()
}

// Invalidate clears all cached data.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = nil
	c.statusTime = time.Time{}
	c.diffs = make(map[string]*FileDiff)
	c.diffTimes = make(map[string]time.Time)
	c.stat = nil
	c.statTime = time.Time{}
}

// InvalidateFile clears cached diff data for a specific file.
func (c *Cache) InvalidateFile(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.diffs, path)
	delete(c.diffTimes, path)
	c.status = nil
	c.statusTime = time.Time{}
	c.stat = nil
	c.statTime = time.Time{}
}
