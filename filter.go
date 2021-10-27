// Based on github.com/riobard/go-bloom
// Apache License 2.0

package disk_bloom

import (
	"encoding/binary"
	"fmt"
	"io"
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

const LenOfMetadataSize = 2

type muFile struct {
	f        *os.File
	fsync    FsyncMode
	modified bool
	mu       sync.Mutex
}

var InconsistentMetadataSizeErr = fmt.Errorf("inconsistent metadata size")

// Disk-based Classic Bloom Filter
type DiskFilter struct {
	param *FilterParam
	file  muFile
	// use this channel to inform the sync goroutine
	closed     chan struct{}
	controller *Controller
}

type FilterParam struct {
	Slots uint8
	Bits  uint64
	Hash  func([]byte) (uint64, uint64)
}

type Controller struct {
	Fsync FsyncMode
	// Size in bytes
	MetadataSize uint16
	// Control will be invoked every second.
	//
	// | len of metadata size(2 bytes) | metadata | bloom filter |
	Control func(f *os.File, modified bool)
	// GetParam will be invoked when New.
	//
	// | len of metadata size(2 bytes) | metadata | bloom filter |
	GetParam func(metadata []byte) (param FilterParam, updatedMetadata []byte)
}

// n is the expected number of entries.
// p is the expected false positive rate.
func OptimalParam(n uint64, p float64) (slots uint8, bits uint64) {
	k := -math.Log(p) * math.Log2E   // number of hashes
	m := float64(n) * k * math.Log2E // number of bits
	return uint8(k + 0.5), uint64(m / 8 * 8)
}

// New creates a classic Bloom Filter.
// h is a double hash that takes an entry and returns two different hashes.
func New(filename string, controller Controller) (*DiskFilter, error) {
	// calculate the optimal num of bits
	mode := os.O_CREATE | os.O_RDWR
	// open the data file
	if controller.Fsync == FsyncModeAlways {
		mode |= os.O_SYNC
	}
	f, err := os.OpenFile(filename, mode, 0644)
	if err != nil {
		return nil, err
	}
	var param FilterParam
	var metadataSize [LenOfMetadataSize]byte
	var updatedMetadata []byte
	if n, err := f.ReadAt(metadataSize[:], 0); n == 0 && err == io.EOF {
		param, updatedMetadata = controller.GetParam(nil)
		// create a new file
		// write at the end of file to allocate specific space in the disk
		// TODO: thick provision?
		if _, err = f.WriteAt([]byte{0}, LenOfMetadataSize+int64(controller.MetadataSize)+int64(param.Bits/8)); err != nil {
			return nil, err
		}
		// write the metadata size at the head of file (2 bytes).
		binary.LittleEndian.PutUint16(metadataSize[:], controller.MetadataSize)
		if _, err = f.WriteAt(metadataSize[:], 0); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if fms := binary.LittleEndian.Uint16(metadataSize[:]); fms != controller.MetadataSize {
		return nil, fmt.Errorf("%w: the metadata size written in the given file is %v, which is different from %v", InconsistentMetadataSizeErr, fms, controller.MetadataSize)
	} else {
		metadata := make([]byte, controller.MetadataSize)
		if _, err := f.ReadAt(metadata[:], 2); err != nil {
			return nil, err
		}
		param, updatedMetadata = controller.GetParam(metadata)
	}
	if updatedMetadata != nil {
		if len(updatedMetadata) != int(controller.MetadataSize) {
			return nil, fmt.Errorf("%w: length of updated metadata can not satisfy", InconsistentMetadataSizeErr)
		}
		if _, err = f.WriteAt(updatedMetadata, LenOfMetadataSize); err != nil {
			return nil, err
		}
	}
	filter := DiskFilter{
		param:      &param,
		file:       muFile{f: f},
		controller: &controller,
		closed:     make(chan struct{}),
	}
	if controller.Fsync == FsyncModeEverySec || controller.Control != nil {
		go filter.eventEverySec()
	}
	return &filter, nil
}

// Close should be invoked if the filter is not needed anymore
func (f *DiskFilter) Close() error {
	select {
	case <-f.closed:
		return nil
	default:
	}
	close(f.closed)
	f.file.mu.Lock()
	defer f.file.mu.Unlock()
	if f.file.fsync != FsyncModeAlways && f.file.modified {
		f.file.modified = false
		_ = f.file.f.Sync()
	}
	_ = f.file.f.Close()
	return nil
}

func (f *DiskFilter) eventEverySec() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		select {
		case <-f.closed:
			ticker.Stop()
			return
		default:
		}
		f.file.mu.Lock()
		if f.file.fsync == FsyncModeEverySec && f.file.modified {
			f.file.modified = false
			_ = f.file.f.Sync()
		}
		if f.controller.Control != nil {
			f.controller.Control(f.file.f, f.file.modified)
		}
		f.file.mu.Unlock()
	}
}

func (f *DiskFilter) bloomOffset(x, y uint64, i int) uint64 {
	return (x + uint64(i)*y) % f.param.Bits
}

// fileOffset returns the fileOffset relative to the beginning of the file
func (f *DiskFilter) fileOffset(bloomOffset int64) int64 {
	return LenOfMetadataSize + int64(f.controller.MetadataSize) + bloomOffset
}

// Exist returns if an entry is in the filter
func (f *DiskFilter) Exist(b []byte) bool {
	x, y := f.param.Hash(b)
	var offsets = make([]uint64, f.param.Slots)
	for i := 0; i < int(f.param.Slots); i++ {
		offsets[i] = f.bloomOffset(x, y, i)
	}
	// sort to improve the performance on HDD
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})
	var m = make(map[int64]byte)
	f.file.mu.Lock()
	defer f.file.mu.Unlock()
	for _, offset := range offsets {
		var b [1]byte
		pos := f.fileOffset(int64(offset / 8))
		if val, ok := m[pos]; ok {
			b[0] = val
		} else {
			f.file.f.ReadAt(b[:], pos)
			m[pos] = b[0]
		}
		if b[0]&(1<<(offset%8)) == 0 {
			return false
		}
	}
	return true
}

// ExistOrAdd returns whether the entry was in the filter, and adds an entry to the filter if it was not in.
func (f *DiskFilter) ExistOrAdd(b []byte) (exist bool) {
	x, y := f.param.Hash(b)
	var offsets = make([]uint64, f.param.Slots)
	for i := 0; i < int(f.param.Slots); i++ {
		offsets[i] = f.bloomOffset(x, y, i)
	}
	// sort to improve the performance on HDD
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})
	var m = make(map[int64]byte)
	f.file.mu.Lock()
	defer f.file.mu.Unlock()
	exist = true
	for _, offset := range offsets {
		var b [1]byte
		pos := f.fileOffset(int64(offset / 8))
		if val, ok := m[pos]; ok {
			b[0] = val
		} else {
			f.file.f.ReadAt(b[:], pos)
			m[pos] = b[0]
		}
		if b[0]&(1<<(offset%8)) == 0 {
			exist = false
		}
		m[pos] |= 1 << (offset % 8)
	}
	if exist {
		return
	}
	for _, offset := range offsets {
		pos := f.fileOffset(int64(offset / 8))
		if val, ok := m[pos]; ok {
			f.file.f.WriteAt([]byte{val}, pos)
			delete(m, pos)
		}
	}
	f.file.modified = true
	return
}

// Size returns the size of the filter in bytes
func (f *DiskFilter) Size() uint64 { return f.param.Bits / 8 }

func (f *DiskFilter) FilterParam() FilterParam {
	return *f.param
}

func (f *DiskFilter) Controller() Controller {
	return *f.controller
}
