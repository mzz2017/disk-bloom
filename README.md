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
    n         = 1e6
    expectFPR = 1e-4
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

## Related

+ https://github.com/shadowsocks/shadowsocks-rust/pull/556

## Thanks

+ The discussion with [moodyhunter](https://github.com/moodyhunter) and [xiaokangwang](https://github.com/xiaokangwang) inspired me.
+ The project [go-bloom](https://github.com/riobard/go-bloom/blob/master/filter.go).