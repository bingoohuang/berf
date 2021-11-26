# perf

perf framework for local methods.

```sh
$ perf -f demo -v
Benchmarking demo using 300 goroutine(s), 12 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed             42.579s
  Count/RPS  974091 22877.032
    200      876973 20596.165
    500        97118 2280.867

Statistics    Min      Mean    StdDev     Max   
  Latency      0s     13.11ms  5.147ms  23.491ms
  RPS       22762.77  22876.6   53.63   22979.41

Latency Percentile:
  P50         P75       P90       P95       P99      P99.9     P99.99 
  14.043ms  17.026ms  18.144ms  19.049ms  19.146ms  19.254ms  20.377ms

Latency Histogram:
  523µs     102229  10.49%  ■■■■■■■■■■■■■
  10.81ms   159974  16.42%  ■■■■■■■■■■■■■■■■■■■■
  12.357ms  168439  17.29%  ■■■■■■■■■■■■■■■■■■■■■
  14.402ms  179576  18.44%  ■■■■■■■■■■■■■■■■■■■■■■
  17.208ms  321710  33.03%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  18.289ms   26520   2.72%  ■■■
  19.075ms   15635   1.61%  ■■
  19.104ms       8   0.00%  
```
