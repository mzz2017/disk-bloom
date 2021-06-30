// Based on github.com/riobard/go-bloom
// Apache License 2.0

package bloom

import (
	"math"
	"os"
	"sort"
	"sync"
	"time"
)

type FsyncMode int

const (
	FsyncModeAlways FsyncMode = iota
	FsyncModeEverySec
	FsyncModeNo
)

type muFile struct {
	f     *os.File
	fsync FsyncMode
	mu    sync.Mutex
}

// Disk-based Classic Bloom Filter
type DiskClassicFilter struct {
	k    int
	m    int64
	h    func([]byte) (uint64, uint64)
	file muFile
	// use this channel to inform the sync goroutine
	closed chan struct{}
}

// New creates a classic Bloom Filter.
// n is the expected number of entries.
// p is the expected false positive rate.
// h is a double hash that takes an entry and returns two different hashes.
func New(path string, fsync FsyncMode, n int, p float64, h func([]byte) (uint64, uint64)) (*DiskClassicFilter, error) {
	// calculate the optimal num of bits
	k := -math.Log(p) * math.Log2E   // number of hashes
	m := float64(n) * k * math.Log2E // number of bits
	mode := os.O_CREATE | os.O_RDWR
	// open the data file
	if fsync == FsyncModeAlways {
		mode |= os.O_SYNC
	}
	f, err := os.OpenFile(path, mode, 0644)
	if err != nil {
		return nil, err
	}
	filter := DiskClassicFilter{
		k:      int(k + 0.5), // rounding
		m:      int64(m),
		h:      h,
		file:   muFile{f: f},
		closed: make(chan struct{}),
	}
	// write at the end of file to allocate specific space in the disk
	// TODO: thick provision?
	_, err = f.WriteAt([]byte{0}, filter.m/8)
	if err != nil {
		return nil, err
	}
	switch fsync {
	case FsyncModeEverySec:
		go filter.syncEverySec()
	}
	return &filter, nil
}

// Close should be invoked if the filter is not needed anymore
func (f *DiskClassicFilter) Close() error {
	select {
	case <-f.closed:
		return nil
	default:
	}
	close(f.closed)
	f.file.mu.Lock()
	defer f.file.mu.Unlock()
	if f.file.fsync != FsyncModeAlways {
		_ = f.file.f.Sync()
	}
	_ = f.file.f.Close()
	return nil
}

func (f *DiskClassicFilter) syncEverySec() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		select {
		case <-f.closed:
			ticker.Stop()
			return
		default:
		}
		f.file.mu.Lock()
		_ = f.file.f.Sync()
		f.file.mu.Unlock()
	}
}

func (f *DiskClassicFilter) getOffset(x, y uint64, i int) uint64 {
	return (x + uint64(i)*y) % (uint64(f.m))
}

// Add adds an entry to the filter
func (f *DiskClassicFilter) Add(b []byte) {
	x, y := f.h(b)
	var offsets []uint64
	for i := 0; i < f.k; i++ {
		offsets = append(offsets, f.getOffset(x, y, i))
	}
	// sort to improve the performance on HDD
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})
	f.file.mu.Lock()
	for _, offset := range offsets {
		var b [1]byte
		pos := int64(offset / 8)
		f.file.f.ReadAt(b[:], pos)
		b[0] |= 1 << (offset % 8)
		f.file.f.WriteAt(b[:], pos)
	}
	f.file.mu.Unlock()
}

// Exist returns if an entry is in the filter
func (f *DiskClassicFilter) Exist(b []byte) bool {
	x, y := f.h(b)
	var offsets []uint64
	for i := 0; i < f.k; i++ {
		offsets = append(offsets, f.getOffset(x, y, i))
	}
	// sort to improve the performance on HDD
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})
	f.file.mu.Lock()
	for _, offset := range offsets {
		var b [1]byte
		pos := int64(offset / 8)
		f.file.f.ReadAt(b[:], pos)
		if b[0]&(1<<(offset%8)) == 0 {
			return false
		}
	}
	f.file.mu.Unlock()
	return true
}

// ExistOrAdd costs less than continuously invoking Exist and Add, and returns whether the entry was in the filter before.
func (f *DiskClassicFilter) ExistOrAdd(b []byte) bool {
	x, y := f.h(b)
	var offsets []uint64
	var newVals [][]byte
	for i := 0; i < f.k; i++ {
		offsets = append(offsets, f.getOffset(x, y, i))
	}
	// sort to improve the performance on HDD
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})
	newVals = make([][]byte, len(offsets))
	f.file.mu.Lock()
	for i, offset := range offsets {
		var b [1]byte
		pos := int64(offset / 8)
		f.file.f.ReadAt(b[:], pos)
		if b[0]&(1<<(offset%8)) == 0 {
			return true
		}
		newVals[i] = []byte{b[0] | 1<<(offset%8)}
	}
	for i, offset := range offsets {
		f.file.f.WriteAt(newVals[i], int64(offset/8))
	}
	f.file.mu.Unlock()
	return false
}

// Size returns the size of the filter in bytes
func (f *DiskClassicFilter) Size() int64 { return f.m / 8 }
