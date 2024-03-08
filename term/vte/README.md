# Benchmarks
These benchmarks are integration benchmarks that can serve as a baseline comparison betwen
terminal emulator implementations.

All tests were performed on Macbook Pro M2 14-inch in 2023, full screen on the macbook's retina screen,
with the given command and file:
```
$ time cat /tmp/aaa 

$ ls -altrh /tmp/aaa
-rw-r--r--@ 1 ernestrc  wheel    44M Mar  6 14:45 /tmp/aaa

```

## Alacritty
```
cat /tmp/aaa  0.00s user 0.15s system 23% cpu 0.652 total
```

## MacOS Terminal app
```
cat /tmp/aaa  0.00s user 0.16s system 13% cpu 1.180 total
```

## Iterm2
```
cat /tmp/aaa  0.00s user 0.24s system 13% cpu 1.828 total
```

## Six running on Alacritty without any optimizations (25f7994c57ca75d8186d68125c7c9e115b12818f)
```
cat /tmp/aaa  0.00s user 0.31s system 0% cpu 2:18.61 total
```

## Six running on Alacritty, version 1.1
Remove all trace logs and initialize scroll and buffer with InitPerformance.
```
cat /tmp/aaa  0.00s user 0.29s system 1% cpu 21.012 total
```

## Six running on Alacritty, version 1.2
Interrupt at most at 30fps, with max scroll length (reduces pressure on GC)
```
cat /tmp/aaa  0.00s user 0.17s system 1% cpu 9.704 total 
```
