# performance on AES-GCM-128

```sh
$ berf-crypto -f alg=sm4 -n1
Input : yliSCincHh56yqTx2tU2v6oxOsRX7lfYsMGBJ936x2R1vW3-uLZ8yBiN1skLgFtjPPTflJZ3ew6d1Pat6ZGTcA (base64.RawURLEncoding)
Key   : 9vp_4npwFPl61E3ibzWGuA (base64.RawURLEncoding)
IV    : CIeOz71eHEQQ5dJewDaO0w (base64.RawURLEncoding)
Base64RawURL: BPr4UvuGEOdAe4oakb6xnqrdW9beoY3Q8yaTBW8sO_1DB-1T4iLFpfKrMvwm_yEfLWDv3SuVT0Kxb36bTygntTBWEPMi7mAnjx0UkCFdzVM
```

```sh
$ berf-crypto -f alg=sm4,input=贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室 -n1
Input : 贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
Key   : KQPif779NCLFzOC9HlLGqg (base64.RawURLEncoding)
IV    : 1T_G1iqcq2w7frfetLsbFw (base64.RawURLEncoding)
Base64RawURL: LqRGzuvpl4dDglCmlUYf1CKt3n1xLhsR-SGO2Pt9j8HunVr9qSyc58b02amFjwsmtQHCoByB0X47sP-jAIRiY30EQ-qqPSbUxpes58jE12THx766kF0FBJJ2DqnFFEci

$ berf-crypto -f decode,alg=sm4,input=LqRGzuvpl4dDglCmlUYf1CKt3n1xLhsR-SGO2Pt9j8HunVr9qSyc58b02amFjwsmtQHCoByB0X47sP-jAIRiY30EQ-qqPSbUxpes58jE12THx766kF0FBJJ2DqnFFEci,key=KQPif779NCLFzOC9HlLGqg,iv=1T_G1iqcq2w7frfetLsbFw -n1
Input : LqRGzuvpl4dDglCmlUYf1CKt3n1xLhsR-SGO2Pt9j8HunVr9qSyc58b02amFjwsmtQHCoByB0X47sP-jAIRiY30EQ-qqPSbUxpes58jE12THx766kF0FBJJ2DqnFFEci (base64.RawURLEncoding)
Key   : KQPif779NCLFzOC9HlLGqg (base64.RawURLEncoding)
IV    : 1T_G1iqcq2w7frfetLsbFw (base64.RawURLEncoding)
Result: 贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
Base64RawURL: 6LS15bee55yB6buU6KW_5Y2X5biD5L6d5peP6IuX5peP6Ieq5rK75bee5qib6Iur6LevMzk1OeWPt-ahmOaGtOWwj-WMujEz5Y2V5YWDMTkyNeWupA
```


```sh
$ export FEATURE_KEY=9vp_4npwFPl61E3ibzWGuA
$ export FEATURE_IV=CIeOz71eHEQQ5dJewDaO0w
$ export FEATURE_INPUT=贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
$ berf-crypto -f alg=sm4 -n1
Input : 贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
Key   : 9vp_4npwFPl61E3ibzWGuA (base64.RawURLEncoding)
IV    : CIeOz71eHEQQ5dJewDaO0w (base64.RawURLEncoding)
Base64RawURL: X-6sWkYfoHdX2N0BAd9g5Sb8BipFAx2SxAysJMvUz5Eubw9eEUlepqltQr7V7FUOEyMiUAhdCMUaixgyQmcZzyeWvrLKRxrJ6Pe5kXgK-O-mPnUm91jV79iY2WjODhBQ
$ export FEATURE_INPUT=X-6sWkYfoHdX2N0BAd9g5Sb8BipFAx2SxAysJMvUz5Eubw9eEUlepqltQr7V7FUOEyMiUAhdCMUaixgyQmcZzyeWvrLKRxrJ6Pe5kXgK-O-mPnUm91jV79iY2WjODhBQ
$ berf-crypto -f alg=sm4,decode -n1
Input : X-6sWkYfoHdX2N0BAd9g5Sb8BipFAx2SxAysJMvUz5Eubw9eEUlepqltQr7V7FUOEyMiUAhdCMUaixgyQmcZzyeWvrLKRxrJ6Pe5kXgK-O-mPnUm91jV79iY2WjODhBQ (base64.RawURLEncoding)
Key   : 9vp_4npwFPl61E3ibzWGuA (base64.RawURLEncoding)
IV    : CIeOz71eHEQQ5dJewDaO0w (base64.RawURLEncoding)
Result: 贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
Base64RawURL: 6LS15bee55yB6buU6KW_5Y2X5biD5L6d5peP6IuX5peP6Ieq5rK75bee5qib6Iur6LevMzk1OeWPt-ahmOaGtOWwj-WMujEz5Y2V5YWDMTkyNeWupA
$ echo X-6sWkYfoHdX2N0BAd9g5Sb8BipFAx2SxAysJMvUz5Eubw9eEUlepqltQr7V7FUOEyMiUAhdCMUaixgyQmcZzyeWvrLKRxrJ6Pe5kXgK-O-mPnUm91jV79iY2WjODhBQ > /tmp/x6s.txt
$ export FEATURE_INPUT=@/tmp/x6s.txt                                                                                                                   
$ berf-crypto -f alg=sm4,decode -n1
Input : X-6sWkYfoHdX2N0BAd9g5Sb8BipFAx2SxAysJMvUz5Eubw9eEUlepqltQr7V7FUOEyMiUAhdCMUaixgyQmcZzyeWvrLKRxrJ6Pe5kXgK-O-mPnUm91jV79iY2WjODhBQ (base64.RawURLEncoding)
Key   : 9vp_4npwFPl61E3ibzWGuA (base64.RawURLEncoding)
IV    : CIeOz71eHEQQ5dJewDaO0w (base64.RawURLEncoding)
Result: 贵州省黔西南布依族苗族自治州樛苫路3959号桘憴小区13单元1925室
Base64RawURL: 6LS15bee55yB6buU6KW_5Y2X5biD5L6d5peP6IuX5peP6Ieq5rK75bee5qib6Iur6LevMzk1OeWPt-ahmOaGtOWwj-WMujEz5Y2V5YWDMTkyNeWupA
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
