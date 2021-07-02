// Based on github.com/riobard/go-bloom
// Apache License 2.0

package disk_bloom

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func TestFilterGroup_Exist(t *testing.T) {
	os.Mkdir("testfile", os.ModePerm)
	bf, _ := NewGroup("testfile/*", FsyncModeEverySec, 1e6, 1e-4, doubleFNV)
	defer func() {
		os.RemoveAll("testfile")
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

func TestFilterGroupFalsePositive(t *testing.T) {
	const (
		n         = 1e6
		expectFPR = 1e-4
		N         = 2 * n
	)
	os.Mkdir("testfile", os.ModePerm)
	bf, _ := NewGroup("testfile/*", FsyncModeEverySec, n, expectFPR, doubleFNV)
	defer func() {
		os.RemoveAll("testfile")
	}()
	samples := make([][]byte, N)
	fp := 0 // false positive count
	fn := 0
	for i := 0; i < N; i++ {
		x := []byte(fmt.Sprint(i))
		samples[i] = x
		if bf.ExistOrAdd(x) {
			fn++
		}
	}

	for _, x := range samples {
		if !bf.Exist(x) {
			fp++
		}
	}
	fpr := float64(fp) / N
	t.Logf("Samples = %d, FP = %d, FPR = %.4f%%, FN = %d", int(N), fp, fpr*100, fn)
}

func BenchmarkFilterGroup_ExistOrAdd(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	os.Mkdir("testfile", os.ModePerm)
	bf, _ := NewGroup("testfile/*", FsyncModeEverySec, 1e6, 1e-4, doubleFNV)
	defer func() {
		os.RemoveAll("testfile")
	}()
	buf := make([]byte, 20)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		binary.PutUvarint(buf, uint64(i))
		bf.ExistOrAdd(buf)
	}
}

func BenchmarkFilterGroup_Exist(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	os.Mkdir("testfile", os.ModePerm)
	bf, _ := NewGroup("testfile/*", FsyncModeEverySec, 1e6, 1e-4, doubleFNV)
	defer func() {
		os.RemoveAll("testfile")
	}()
	buf := make([]byte, 20)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		binary.PutUvarint(buf, uint64(i))
		bf.Exist(buf)
	}
}
