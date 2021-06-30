# disk-bloom-filter

Operate the bloom filter on the disk for long-term anti-replay protection.

```golang
func doubleFNV(b []byte) (uint64, uint64) {
    hx := fnv.New64()
    hx.Write(b)
    x := hx.Sum64()
    hy := fnv.New64a()
    hy.Write(b)
    y := hy.Sum64()
    return x, y
}

bf, err := bloom.New("example.dbf", bloom.FsyncModeEverySec, 1000000, 0.0001, doubleFNV)
if err != nil {
    panic(err)
}
bf.Add([]byte("hello"))
bf.Exist([]byte("hello"))
bf.ExistOrAdd([]byte("world"))
```

## Thanks

+ The discussion with [moodyhunter](https://github.com/moodyhunter) and [xiaokangwang](https://github.com/xiaokangwang) inspired me.
+ The project [go-bloom](https://github.com/riobard/go-bloom/blob/master/filter.go).