# changes

## 2023年03月28日 国密 TLCP 支持

```shell
$ export TLCP=true
$ berf https://10.0.0.18:15443 -d1m
Berf benchmarking https://10.0.0.18:15443/ for 1m0s using 100 goroutine(s), 6 GoMaxProcs.

Summary:
  Elapsed             1m0.026s
  Count/RPS    375621 6257.597
    200        375621 6257.597
  ReadWrite  16.108 9.764 Mbps

Statistics   Min      Mean     StdDev      Max
  Latency   160µs   15.949ms  12.852ms  563.238ms
  RPS       1695.6  6253.85   1784.42    9146.11

Latency Percentile:
  P50         P75      P90       P95       P99       P99.9     P99.99
  12.634ms  18.24ms  27.602ms  37.636ms  62.337ms  133.806ms  332.125ms
$
$ berf http://10.0.0.18:15080 -d1m
Berf benchmarking http://10.0.0.18:15080/ for 1m0s using 100 goroutine(s), 6 GoMaxProcs.

Summary:
  Elapsed              1m0.003s
  Count/RPS   1399237 23319.295
    200       1399237 23319.295
  ReadWrite  50.547 30.408 Mbps

Statistics    Min      Mean    StdDev     Max
  Latency    48µs    4.272ms   5.23ms  223.739ms
  RPS       6051.15  23301.69  8105.9  33234.89

Latency Percentile:
  P50        P75      P90      P95      P99      P99.9     P99.99
  2.817ms  4.085ms  7.136ms  11.25ms  29.208ms  58.229ms  112.992ms
```

```sh
$ export TLCP=true
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
    keepalive_timeout  65;


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
