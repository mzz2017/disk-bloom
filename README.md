# disk-bloom-filter

Disk-based bloom filter. It operates the bloom filter on the disk for long-term anti-replay protection.

```golang
func doubleFNVFactory(salt []byte) func (b []byte) (uint64, uint64) {
	return func(b []byte) (uint64, uint64) {
		hx := fnv.New64()
		hx.Write(b)
		hx.Write(salt)
		x := hx.Sum64()
		hy := fnv.New64a()
		hy.Write(b)
		hy.Write(salt)
		y := hy.Sum64()
		return x, y
	}
}

const (
    n         = 1e8
    expectFPR = 1e-6
)

bf, err := disk_bloom.NewGroup("testfile*", disk_bloom.FsyncModeEverySec, n, expectFPR, doubleFNVFactory([]byte("some_salt")))
if err != nil {
    panic(err)
}
bf.ExistOrAdd([]byte("hello"))
bf.Exist([]byte("hello"))
bf.ExistOrAdd([]byte("world"))
```

Note that it is not recommended to use doubleFNV directly, please be sure to add user-personalized salt to prevent active detection attacks based on hash collisions.

## Benchmark
```
goos: linux
goarch: amd64
cpu: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
disk: LENSE20256GMSP34MEAT2TA NVME

pkg: github.com/mzz2017/disk-bloom
BenchmarkFilterGroup_ExistOrAdd
BenchmarkFilterGroup_ExistOrAdd-8   	   32413	     33660 ns/op	     521 B/op	       9 allocs/op
BenchmarkFilterGroup_Exist
BenchmarkFilterGroup_Exist-8        	  631995	      1871 ns/op	     304 B/op	       7 allocs/op
BenchmarkDiskFilter_ExistOrAdd
BenchmarkDiskFilter_ExistOrAdd-8    	   34406	     34097 ns/op	     520 B/op	       9 allocs/op
BenchmarkDiskFilter_Exist
BenchmarkDiskFilter_Exist-8         	  626137	      1857 ns/op	     304 B/op	       7 allocs/op

pkg: github.com/riobard/go-bloom
BenchmarkAdd
BenchmarkAdd-8    	 5635690	       186.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkTest
BenchmarkTest-8   	15413245	        74.68 ns/op	       0 B/op	       0 allocs/op
```

## Related

+ https://github.com/shadowsocks/shadowsocks-rust/pull/556

## Thanks

+ The discussion with [moodyhunter](https://github.com/moodyhunter) and [xiaokangwang](https://github.com/xiaokangwang) inspired me.
+ The project [go-bloom](https://github.com/riobard/go-bloom/blob/master/filter.go).