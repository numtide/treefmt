package format

import (
	"fmt"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gobwas/glob"
)

type MatcherCacheKey int

const (
	GlobCacheKey MatcherCacheKey = iota
	MimetypeCacheKey
)

type MatcherCache map[MatcherCacheKey]*sync.Map

func NewMatcherCache() MatcherCache {
	return MatcherCache{}
}

func (cache MatcherCache) Subcache(key MatcherCacheKey) *sync.Map {
	subcache := cache[key]

	if subcache == nil {
		subcache = &sync.Map{}
		cache[key] = subcache
	}

	return subcache
}

func MatcherWants(m Matcher, path string, cache MatcherCache) bool {
	if m.Ignore() {
		return true
	}

	match := m.MatchesPath(path, cache)

	if m.Invert() {
		return !match
	}

	return match
}

type Matcher interface {
	MatchesPath(path string, cache MatcherCache) bool
	Ignore() bool
	Invert() bool
	CacheKey() MatcherCacheKey
}

type inclusionMatcher struct{}

func (*inclusionMatcher) Invert() bool {
	return false
}

type exclusionMatcher struct{}

func (*exclusionMatcher) Invert() bool {
	return true
}

type globMatcher struct {
	globs    []glob.Glob
	patterns []string
}

func newGlobMatcher(patterns []string) (*globMatcher, error) {
	globs := make([]glob.Glob, len(patterns))

	for i, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile include pattern '%v': %w", pattern, err)
		}

		globs[i] = g
	}

	return &globMatcher{globs: globs, patterns: patterns}, nil
}

func (gm *globMatcher) Ignore() bool {
	return len(gm.globs) < 1
}

func (gm *globMatcher) MatchesPath(path string, _ MatcherCache) bool {
	for _, g := range gm.globs {
		if g.Match(path) {
			return true
		}
	}

	return false
}

func (gm *globMatcher) CacheKey() MatcherCacheKey {
	return GlobCacheKey
}

type GlobInclusionMatcher struct {
	*inclusionMatcher
	globMatcher
}

func NewGlobInclusionMatcher(patterns []string) (*GlobInclusionMatcher, error) {
	gm, err := newGlobMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return &GlobInclusionMatcher{globMatcher: *gm}, nil
}

type GlobExclusionMatcher struct {
	*exclusionMatcher
	globMatcher
}

func NewGlobExclusionMatcher(patterns []string) (*GlobExclusionMatcher, error) {
	gm, err := newGlobMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return &GlobExclusionMatcher{globMatcher: *gm}, nil
}

type mimetypeCacheEntry struct {
	mime *mimetype.MIME
	err  error
}

// Ensure that we check a given file at most once (note that there is a is a
// race condition here if the cache isn't yet populated for a given path).
func mimetypeOfPath(path string, subcache *sync.Map) (*mimetype.MIME, error) {
	if cached, ok := subcache.Load(path); ok {
		if entry, ok := cached.(mimetypeCacheEntry); ok {
			return entry.mime, entry.err
		}
	}

	mime, err := mimetype.DetectFile(path)
	if err != nil {
		err = fmt.Errorf("failed to detect MIME type of path '%s': '%w'", path, err)
	}

	subcache.Store(path, mimetypeCacheEntry{
		mime: mime,
		err:  err,
	})

	return mime, err
}

type mimetypeMatcher struct {
	mimes []string
}

func (mm *mimetypeMatcher) Ignore() bool {
	return len(mm.mimes) < 1
}

func (mm *mimetypeMatcher) CacheKey() MatcherCacheKey {
	return MimetypeCacheKey
}

func (mm *mimetypeMatcher) MatchesPath(path string, cache MatcherCache) bool {
	mime, err := mimetypeOfPath(path, cache.Subcache(mm.CacheKey()))
	if err != nil {
		return false
	}

	return mimetype.EqualsAny(mime.String(), mm.mimes...)
}

type MimetypeInclusionMatcher struct {
	*inclusionMatcher
	mimetypeMatcher
}

func NewMimetypeInclusionMatcher(mimes []string) (*MimetypeInclusionMatcher, error) {
	return &MimetypeInclusionMatcher{mimetypeMatcher: mimetypeMatcher{mimes: mimes}}, nil
}

type MimetypeExclusionMatcher struct {
	*exclusionMatcher
	mimetypeMatcher
}

func NewMimetypeExclusionMatcher(mimes []string) (*MimetypeExclusionMatcher, error) {
	return &MimetypeExclusionMatcher{mimetypeMatcher: mimetypeMatcher{mimes: mimes}}, nil
}

type CompositeMatcher struct {
	matchers []Matcher
}

func NewCompositeMatcher(matchers []Matcher) *CompositeMatcher {
	return &CompositeMatcher{matchers: matchers}
}

func (cm *CompositeMatcher) Wants(path string, cache MatcherCache) bool {
	for _, matcher := range cm.matchers {
		if !MatcherWants(matcher, path, cache) {
			return false
		}
	}

	return true
}
