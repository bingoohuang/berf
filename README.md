# perf

perf framework for local methods.

```sh
$ perf -f demo -v -ci 1 -cd 3s -c10
Benchmarking demo using max 10 goroutine(s), 12 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Concurrent              0
  Elapsed          1m0.001s
  Count/RPS   24753 412.537
    200       22362 372.688
    500         2391 39.849

Statistics   Min     Mean    StdDev     Max
  Latency    1µs   13.328ms  5.154ms  31.093ms
  RPS       68.94   412.53   219.73    761.04

Latency Percentile:
  P50         P75       P90       P95       P99      P99.9     P99.99
  14.147ms  17.102ms  18.559ms  19.153ms  19.579ms  20.199ms  30.011ms

Latency Histogram:
  17µs      2159   8.72%  ■■■■■■■■■■■■■■■■■■■■
  9.789ms   2840  11.47%  ■■■■■■■■■■■■■■■■■■■■■■■■■■
  11.773ms  3839  15.51%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  13.504ms  3965  16.02%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  15.073ms  3269  13.21%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  16.908ms  4279  17.29%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  18.55ms   4347  17.56%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  19.462ms    55   0.22%  ■
```

![img.png](_doc/img.png)
