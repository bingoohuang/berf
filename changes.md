# changes

## 2023年11月28日 验证代理使用 (http_proxy 等环境变量， PROXY 环境变量)

1. 优先 PROXY 环境变量， e.g. PROXY=:7890
2. PROXY=0 关闭代理
3. 环境代理（http_proxy、https_proxy等）
    
## 2023年05月25日 文件上传支持文件名添加文件序号

`export UPLOAD_INDEX=.%y%M%d.%H%m%s.%i`

/Users/bingoo/Downloads/20230523194821.27.jpg 就会变成 /Users/bingoo/Downloads/20230523194821.27.20230525.221822.1.jpg

在压测 BeeFS 时，可以起到唯一文件名的作用

## 2023年03月28日 国密 TLCP 支持

| #   | 非 HTTPS | 普通 HTTPS | 国密 HTTPS |
|-----|---------|----------|----------|
| TPS | 28610   | 18629    | 12913    |

```shell
# berf http://192.168.126.18:15080 -d1m -v
Berf benchmarking http://192.168.126.18:15080/ for 1m0s using 100 goroutine(s), 6 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                1m0.021s
  Count/RPS     1717246 28610.673
    200         1717246 28610.673
  ReadWrite    67.750 37.308 Mbps
  Connections                 100

Statistics    Min      Mean    StdDev     Max
  Latency    47µs     3.47ms   3.059ms  71.301ms
  RPS       7593.42  28609.98  4547.22  36912.29

Latency Percentile:
  P50        P75      P90      P95      P99      P99.9     P99.99
  2.664ms  3.385ms  5.514ms  7.717ms  17.208ms  34.879ms  55.648ms

Latency Histogram:
  3.123ms   1601041  93.23%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  5.832ms     90266   5.26%  ■■
  14.313ms    22280   1.30%  ■
  27.113ms     2787   0.16%
  40.881ms      685   0.04%
  48.552ms      120   0.01%
  57.139ms       41   0.00%
  67.094ms       26   0.00%
```

```shell
[root@fs03-192-168-126-18 ~]# berf https://192.168.126.18:22443 -d1m -v
Berf benchmarking https://192.168.126.18:22443/ for 1m0s using 100 goroutine(s), 6 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                 1m0.02s
  Count/RPS     1118132 18629.179
    200         1118132 18629.179
  ReadWrite    36.366 28.621 Mbps
  Connections                 100

Statistics    Min      Mean    StdDev     Max
  Latency    63µs    5.341ms   5.07ms  137.591ms
  RPS       3635.83  18622.69  4672.6   27290.8

Latency Percentile:
  P50      P75      P90      P95       P99      P99.9     P99.99
  3.89ms  4.97ms  8.622ms  13.606ms  28.897ms  49.535ms  72.875ms

Latency Histogram:
  3.905ms   794156  71.03%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  5.357ms   187034  16.73%  ■■■■■■■■■
  9.325ms    88313   7.90%  ■■■■
  18.338ms   37991   3.40%  ■■
  29.627ms    8457   0.76%
  40.143ms    1585   0.14%
  50.077ms     434   0.04%
  74.298ms     162   0.01%
```

```sh
[root@fs03-192-168-126-18 ~]# TLCP=1 berf https://192.168.126.18:15443 -d1m -v
Berf benchmarking https://192.168.126.18:15443/ for 1m0s using 100 goroutine(s), 6 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                1m0.006s
  Count/RPS      774891 12913.553
    200          774891 12913.553
  ReadWrite    34.419 19.839 Mbps
  Connections                 100

Statistics    Min      Mean    StdDev      Max
  Latency    85µs    7.716ms   6.768ms  526.706ms
  RPS       4576.45  12910.14  3512.5   18813.13

Latency Percentile:
  P50        P75      P90       P95       P99      P99.9     P99.99
  5.519ms  8.471ms  13.801ms  19.202ms  33.546ms  59.524ms  224.998ms

Latency Histogram:
  6.371ms   687701  88.75%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  14.956ms   71040   9.17%  ■■■■
  28.355ms   11669   1.51%  ■
  40.632ms    3239   0.42%
  53.427ms     861   0.11%
  65.306ms     285   0.04%
  75.481ms      55   0.01%
  89.203ms      41   0.01%
```

```sh
$ export TLCP=1
$ berf http://10.0.0.18:15080 -n1 -pdb


tengine ntls test OK, ssl_protocol is  (NTLSv1.1 表示国密，其他表示国际)





tengine ntls test OK, ssl_protocol is  (NTLSv1.1 表示国密，其他表示国际)



$ berf https://10.0.0.18:15443 -n1 -pdb
[write] Client Hello, len=47, success=true
>>> ClientHello
Random: bytes=642296d7e18ffb439573a2143f17f2155bca510fcff86c851d17c31bcb4c2003
Session ID:
Cipher Suites: ECC_SM4_GCM_SM3, ECC_SM4_CBC_SM3,
Compression Methods: [0]
<<<
[read] Server Hello, len=74
>>> ServerHello
Random: bytes=547376d21f4416fc3555ac8139a7683f934dea94220caaef444f574e47524400
Session ID: 8fa199ef67097315bd68573ec5e96b374da0c7f54d7857117005ac5c2ef059e5
Cipher Suite: ECC_SM4_GCM_SM3
Compression Method: 0
<<<
[read] Certificate, len=1120
>>> Certificates
Cert[0]:
-----BEGIN CERTIFICATE-----
MIICJzCCAcygAwIBAgIUfHbYwHKmQURFdSxsYr8pc2EJUncwCgYIKoEcz1UBg3Uw
gYIxCzAJBgNVBAYTAkNOMQswCQYDVQQIDAJCSjEQMA4GA1UEBwwHSGFpRGlhbjEl
MCMGA1UECgwcQmVpamluZyBKTlRBIFRlY2hub2xvZ3kgTFRELjEVMBMGA1UECwwM
U09SQiBvZiBUQVNTMRYwFAYDVQQDDA1UZXN0IENBIChTTTIpMB4XDTE5MDUyMzAy
NDU0OFoXDTIzMDcwMTAyNDU0OFowgYYxCzAJBgNVBAYTAkNOMQswCQYDVQQIDAJC
SjEQMA4GA1UEBwwHSGFpRGlhbjElMCMGA1UECgwcQmVpamluZyBKTlRBIFRlY2hu
b2xvZ3kgTFRELjEVMBMGA1UECwwMQlNSQyBvZiBUQVNTMRowGAYDVQQDDBFzZXJ2
ZXIgc2lnbiAoU00yKTBZMBMGByqGSM49AgEGCCqBHM9VAYItA0IABKLD833Sm2zL
epLM5z0EnkYPNJJpTPIBkiDKMkEtqAP5B2D3PFHVqGsfqd5U+nNU1g0/1NPAVY4/
Bio1XRUSQhejGjAYMAkGA1UdEwQCMAAwCwYDVR0PBAQDAgbAMAoGCCqBHM9VAYN1
A0kAMEYCIQDjYHcHhHgjYbQKC8QVYaiix2rgZ/u6i2CcOpF5tpSpLwIhAIgHsYRz
744eAjIlV2oL5t1yDFeWwgVmwn+Z4bGxx4mz
-----END CERTIFICATE-----
Cert[1]:
-----BEGIN CERTIFICATE-----
MIICJDCCAcugAwIBAgIUfHbYwHKmQURFdSxsYr8pc2EJUngwCgYIKoEcz1UBg3Uw
gYIxCzAJBgNVBAYTAkNOMQswCQYDVQQIDAJCSjEQMA4GA1UEBwwHSGFpRGlhbjEl
MCMGA1UECgwcQmVpamluZyBKTlRBIFRlY2hub2xvZ3kgTFRELjEVMBMGA1UECwwM
U09SQiBvZiBUQVNTMRYwFAYDVQQDDA1UZXN0IENBIChTTTIpMB4XDTE5MDUyMzAy
NDU0OFoXDTIzMDcwMTAyNDU0OFowgYUxCzAJBgNVBAYTAkNOMQswCQYDVQQIDAJC
SjEQMA4GA1UEBwwHSGFpRGlhbjElMCMGA1UECgwcQmVpamluZyBKTlRBIFRlY2hu
b2xvZ3kgTFRELjEVMBMGA1UECwwMQlNSQyBvZiBUQVNTMRkwFwYDVQQDDBBzZXJ2
ZXIgZW5jIChTTTIpMFkwEwYHKoZIzj0CAQYIKoEcz1UBgi0DQgAEV1zefsVNw8/F
+Wnbb4SlYrI4Gdfqq9CfdBIACMLpOfzbJT/y0mwSYe7JLovuvXiluMURd8Z4YfxO
vQoXaIcscKMaMBgwCQYDVR0TBAIwADALBgNVHQ8EBAMCAzgwCgYIKoEcz1UBg3UD
RwAwRAIgCJMHFkOqjFWmLB4kzeuRYnffCv0g3vSkKsTlVsAWPFcCIC4A3QVFtQxv
HeHmS/swFJYT+LXSEcIqPksbv1vYItL1
-----END CERTIFICATE-----
<<<
[read] Server Key Exchange, len=78
[read] Server Hello Done, len=4
[write] Client Key Exchange, len=161, success=true
[write] Finished, len=16, success=true
>>> Finished
verify_data: [106 120 109 243 69 203 114 80 58 146 76 142]
<<<
[read] Finished, len=16
>>> Finished
verify_data: [195 42 14 114 156 185 112 106 129 190 120 117]
<<<


tengine ntls test OK, ssl_protocol is NTLSv1.1 (NTLSv1.1 表示国密，其他表示国际)





tengine ntls test OK, ssl_protocol is NTLSv1.1 (NTLSv1.1 表示国密，其他表示国际)



$
```

```nginx
#user  nobody;
worker_processes  1;

error_log  logs/error.log debug;

events {
    worker_connections  1024;
}


http {
    include       mime.types;
    default_type  application/octet-stream;
    sendfile        on;

    keepalive_timeout  120s 120s;
    keepalive_requests 1000000;

    server {
        listen  22443 ssl;
    
        ssl_certificate server.crt;        # 这里为服务器上server.crt的路径
        ssl_certificate_key server.key;    # 这里为服务器上server.key的路径
        #ssl_client_certificate ca.crt;    # 双向认证
        #ssl_verify_client on;             # 双向认证
    
        ssl_session_timeout 5m;
        ssl_protocols SSLv2 SSLv3 TLSv1.1 TLSv1.2;
        ssl_ciphers  ALL:!ADH:!EXPORT56:RC4+RSA:+HIGH:+MEDIUM:+LOW:+SSLv2:+EXP;
        ssl_prefer_server_ciphers   on;
    
        default_type            text/plain;
        add_header  "Content-Type" "text/html;charset=utf-8";

        location / {
            return 200 "SSL";
        }
    }

    server {
        listen       15080;
        listen       15443 ssl;
        ssl_verify_client off;
      	# 国密套件
      	#ssl_ciphers "ECC-SM2-SM4-CBC-SM3:ECDHE-SM2-WITH-SM4-SM3:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-SHA:AES128-GCM-SHA256:AES128-SHA256:AES128-SHA:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:AES256-GCM-SHA384:AES256-SHA256:AES256-SHA:ECDHE-RSA-AES128-SHA256:!aNULL:!eNULL:!RC4:!EXPORT:!DES:!3DES:!MD5:!DSS:!PKS";

        ssl_ciphers "ECC-SM2-SM4-GCM-SM3:ECC-SM2-SM4-CBC-SM3:ECDHE-SM2-SM4-GCM-SM3:ECDHE-SM2-SM4-CBC-SM3";
        ssl_protocols TLSv1 TLSv1.1 TLSv1.2 TLSv1.3;

        default_type            text/plain;
        add_header  "Content-Type" "text/html;charset=utf-8";

        enable_ntls  on;

        # 国密签名证书
        ssl_sign_certificate            SS.cert.pem;
        ssl_sign_certificate_key        SS.key.pem;

        # 国密加密证书
        ssl_enc_certificate             SE.cert.pem;
        ssl_enc_certificate_key         SE.key.pem;

        location / {
            return 200 "tengine ntls test OK, ssl_protocol is $ssl_protocol (NTLSv1.1 表示国密，其他表示国际)";
        }


        error_page   500 502 503 504  /50x.html;
        location = /50x.html {
            root   html;
        }
    }
}
```
