# berf

berf framework for local methods.

## Features

1. LOCAL_IP=ip1,ip2 berf ... 指定网卡 IP 运行 berf
2. support vars substitution
   like `berf -opt eval -n1 192.168.126.5:9200/person/doc/@ksuid  -b '{"addr":"@地址","idcard":"@身份证","name":"@姓名","sex":"@性别"}'`
3. support `-u rand.png,rand.jpg,rand.json` random image and json uploading content.
4. support @var in URL like `berf :9335/pingoo/@ksuid -u imgs -d60s -v`, 2021-12-29
5. support httpie like args `berf :10014/query q="show databases" -n1`, 2021-12-23
6. `berf :5003/api/demo -n20 -pA` to print all details instead of realtime statistics on terminal, 2021-12-22.
7. Add a TPS-0 comparing series to the TPS plots, 2021-12-02.

## Demo

```sh
$ berf -f demo -v -ci 1 -c10
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

## 部署

1. 上传 berf 到 `/usr/local/bin` 目录
2. 建一个 berf 目录，放入 berf 程序: `mkdir berf; cd berf`
3. 在 berf 目录中创建 ctl 脚本: `berf -init`
4. `./ctl start -f nop` 启动硬件监控采点后台进程
5. 下载目录中最新的形如 `berf_202111301122.log.gz` 采点日志
6. 本地使用命令 `berf berf_202111301122.log.gz` 在浏览器中查看采点曲线，进行分析
7. 参数
    - `export BERF_TICK=1s` 每 1s 生成一个点，默认 5s 生成一个.

## Profile 支持

1. 生成 Profile 示例： `berf -P demo.http:new`
2. 编辑新生成的 `demo.http` 文件，如下所示

   ```
   ### 生产环境 env: prod
   export baseURL=http://1.2.3.4:5004
   ### 测试环境 env: test
   export baseURL=http://192.168.0.1:5004
   ### 本地环境 env: local
   export baseURL=http://127.0.0.1:5004
   
   ### [tag=1]
   GET ${baseURL}/status
   
   ### [tag=2]
   POST ${baseURL}/dynamic/demo
   
   {"name": "bingoo"}
   ```

3. 使用 Profile 中的 test 环境变量 跑压测 `berf -P demo.http -env test`
    1. 或者指定 tag 跑压测 `berf -P demo.http:tag1`
    2. 或者指定多个 tag 跑压测 `berf -P demo.http:tag1,tag2`
    3. 或者指定 tag 范围 跑压测 `berf -P demo.http:tag1-tag3`
    4. 混合模式 `berf -P demo.http:tag1-tag3,tag5,tag7-tag9`

## Similar tools

1. [fortio](https://github.com/fortio/fortio)
2. [字节跳动 cloudwego/hertz](https://github.com/cloudwego/hertz)
3. [Vegeta](https://github.com/tsenart/vegeta) is a versatile HTTP load testing tool built out of a need to drill HTTP
   services with a constant request rate. It can be used both as a command line utility and a library.

