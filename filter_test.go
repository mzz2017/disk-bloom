// Based on github.com/riobard/go-bloom
// Apache License 2.0

package disk_bloom

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
	"testing"
)

func doubleFNV(b []byte) (uint64, uint64) {
	hx := fnv.New64()
	hx.Write(b)
	x := hx.Sum64()
	hy := fnv.New64a()
	hy.Write(b)
	y := hy.Sum64()
	return x, y
}

func TestDiskFilter_Exist(t *testing.T) {
	bf, _ := New("testfile", Controller{
		Fsync:        FsyncModeEverySec,
		MetadataSize: 0,
		Control:      nil,
		GetParam: func(metadata []byte) (FilterParam, []byte) {
			slots, bits := OptimalParam(1e6, 1e-4)
			return FilterParam{
				Slots: slots,
				Bits:  bits,
				Hash:  doubleFNV,
			}, nil
		},
	})
	defer func() {
		os.Remove("testfile")
	}()
	buf := []byte("testing")
	bf.ExistOrAdd(buf)
	if !bf.Exist(buf) {
		t.Fatal("Should exist in filter but got false")
	}
	if bf.Exist([]byte("not-exists")) {
		t.Fatal("Should missing in filter but got true")
	}
}

func TestDiskFilterFalsePositive(t *testing.T) {
	const (
		n         = 1e6
		expectFPR = 1e-4
	)
	bf, _ := New("testfile", Controller{
		Fsync:        FsyncModeEverySec,
		MetadataSize: 0,
		Control:      nil,
		GetParam: func(metadata []byte) (FilterParam, []byte) {
			slots, bits := OptimalParam(n, expectFPR)
			return FilterParam{
				Slots: slots,
				Bits:  bits,
				Hash:  doubleFNV,
			}, nil
		},
	})
	defer func() {
		os.Remove("testfile")
	}()
	samples := make([][]byte, n)
	fp := 0 // false positive count
	fn := 0

	for i := 0; i < n; i++ {
		x := []byte(fmt.Sprint(i))
		samples[i] = x
		if bf.ExistOrAdd(x) {
			fp++ // FIXME: not an accurate method
		}
	}

	for _, x := range samples {
		if !bf.Exist(x) {
			fn++
		}
	}
	fpr := float64(fp) / n
	t.Logf("Samples = %d, FP = %d, FPR = %.4f%%, FN = %d", int(n), fp, fpr*100, fn)
}

func BenchmarkDiskFilter_ExistOrAdd(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	bf, _ := New("testfile", Controller{
		Fsync:        FsyncModeEverySec,
		MetadataSize: 0,
		Control:      nil,
		GetParam: func(metadata []byte) (FilterParam, []byte) {
			slots, bits := OptimalParam(1e6, 1e-4)
			return FilterParam{
				Slots: slots,
				Bits:  bits,
				Hash:  doubleFNV,
			}, nil
		},
	})
	defer func() {
		os.Remove("testfile")
	}()
	buf := make([]byte, 20)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		binary.PutUvarint(buf, uint64(i))
		bf.ExistOrAdd(buf)
	}
}

func BenchmarkDiskFilter_Exist(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	bf, _ := New("testfile", Controller{
		Fsync:        FsyncModeEverySec,
		MetadataSize: 0,
		Control:      nil,
		GetParam: func(metadata []byte) (FilterParam, []byte) {
			slots, bits := OptimalParam(1e6, 1e-4)
			return FilterParam{
				Slots: slots,
				Bits:  bits,
				Hash:  doubleFNV,
			}, nil
		},
	})
	defer func() {
		os.Remove("testfile")
	}()
	buf := make([]byte, 20)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		binary.PutUvarint(buf, uint64(i))
		bf.Exist(buf)
	}
}
