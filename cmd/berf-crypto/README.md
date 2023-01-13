# performance on AES-GCM-128

```sh
$ berf-crypto -f alg=sm4 -n1
Plain : VBKtvv6zfw0WgTbqKPyHzt3OefBYOZam1Sl44atFh8xFKunFVdFGIYdV06eR8kX5RKEnG+eAS1Q6JADges7zKQ==
Key   : 3Z6ezhPRC+LgmIgrdM2/Sg==
IV    : PCJETWOdUAiH7l0mehvh5Q==
Result: U83fL7mTPsXUixy06e8mqHPeEk6kDBBNYwYLcxT35L53GqVqNfH4mrd9ftOhQKnVP1C0FlOTQvYoggSs2Qr9SaPPpJhI5dxTh3A03adWy70=
```
```sh
$ go/bin/berf-crypto -f alg=sm4,plain=VBKtvv6zfw0WgTbqKPyHzt3OefBYOZam1Sl44atFh8xFKunFVdFGIYdV06eR8kX5RKEnG+eAS1Q6JADges7zKQ,key=3Z6ezhPRC+LgmIgrdM2/Sg,iv=PCJETWOdUAiH7l0mehvh5Q -n1
Plain : VBKtvv6zfw0WgTbqKPyHzt3OefBYOZam1Sl44atFh8xFKunFVdFGIYdV06eR8kX5RKEnG+eAS1Q6JADges7zKQ==
Key   : 3Z6ezhPRC+LgmIgrdM2/Sg==
IV    : PCJETWOdUAiH7l0mehvh5Q==
Result: U83fL7mTPsXUixy06e8mqHPeEk6kDBBNYwYLcxT35L53GqVqNfH4mrd9ftOhQKnVP1C0FlOTQvYoggSs2Qr9SaPPpJhI5dxTh3A03adWy70=
```

```sh
$ go/bin/berf-crypto -f alg=sm4 -d15s
Berf benchmarking profiles SM4 (alg=sm4) for 15s using 100 goroutine(s), 4 GoMaxProcs.

Summary:
  Elapsed                 15.004s
  Count/RPS   12225129 814773.004
  ReadWrite  417.164 521.455 Mbps

Statistics     Min       Mean      StdDev      Max
  Latency      2µs       20µs      436µs    48.261ms
  RPS       761756.82  814392.96  16140.53  833929.89

Latency Percentile:
  P50  P75  P90  P95  P99   P99.9   P99.99
  2µs  2µs  3µs  4µs  9µs  4.847ms  19.75ms
```

```sh
$ go/bin/berf-crypto -f alg=aes -d15s
Berf benchmarking profiles AES (alg=aes) for 15s using 100 goroutine(s), 4 GoMaxProcs.

Summary:
  Elapsed                 15.005s
  Count/RPS   12658108 843564.648
  ReadWrite  411.660 701.846 Mbps

Statistics     Min       Mean     StdDev      Max
  Latency      0s        24µs     352µs    48.175ms
  RPS       769166.77  842895.9  23349.23  862276.67

Latency Percentile:
  P50  P75  P90  P95   P99    P99.9   P99.99
  0s   1µs  2µs  5µs  340µs  4.049ms  15.53ms
```
