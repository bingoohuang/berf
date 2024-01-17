# 国密测试


环境： tengine 2.4.0 tongsuo 8.3.2

1. 单向 TLS  `berf https://192.168.136.114/v -H Host:www.xyz.com -n1`
2. 单向 TLCP 双证 `TLCP=1 berf https://192.168.136.114/v -H Host:www.xyz.com -n1`
3. 双向 TLCP
   双证 `TLCP_CERTS=sm2_client_sign.crt,sm2_client_sign.key,sm2_client_enc.crt,sm2_client_enc.key TLCP=1 berf https://192.168.136.114/v -H Host:www.abc.com -n1`

性能对比

| 项目              | TPS | 
|-----------------|-----|
| 国密双向双证 ｜ 21589  |
| 国密单向双证 ｜ 22052  |
| TLSv1.3 ｜ 38266 |

## 单向 TLS

```sh
[root@jmjs-PC:/home/zys/tlcp]# berf https://192.168.136.114/v -n1 -H Host:www.xyz.com
option Conn type: *tls.Conn
option TLS.Version: TLSv13
option TLS.Subject: CN=ecc_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLS.KeyUsage: [DigitalSignature ContentCommitment]
option TLS.HandshakeComplete: true
option TLS.DidResume: false
option TLS.CipherSuite: &{ID:4866 Name:TLS_AES_256_GCM_SHA384 SupportedVersions:[772] Insecure:false}
### 192.168.136.114:44446->192.168.136.114:443 时间: 2024-01-17T16:18:08.531466439+08:00 耗时: 2.16746ms  读/写: 2294/540 字节
GET /v HTTP/1.1
User-Agent: blow
Host: www.xyz.com
Content-Type: plain/text; charset=utf-8
Accept: application/json
Accept-Encoding: gzip, deflate


HTTP/1.1 200 OK
Server: Tengine/2.4.0
Date: Wed, 17 Jan 2024 08:18:08 GMT
Content-Type: application/octet-stream
Content-Length: 105
Connection: keep-alive

www.xyz.com 单向SSL通信 test OK, ssl_protocol is TLSv1.3 (NTLSv1.1 表示国密，其他表示国际)
```

## 单向 TLCP 双证

```sh
[root@jmjs-PC:/home/zys/tlcp]# TLCP=1 berf https://192.168.136.114/v -n1 -H Host:www.xyz.com
option Conn type: *tlcp.Conn
option TLCP.Version: TLCP
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [DigitalSignature ContentCommitment]
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [KeyEncipherment DataEncipherment KeyAgreement]
option TLCP.HandshakeComplete: true
option TLCP.DidResume: false
option TLCP.CipherSuite: &{ID:57363 Name:ECC_SM4_CBC_SM3 SupportedVersions:[257] Insecure:false}
### 192.168.136.114:44444->192.168.136.114:443 时间: 2024-01-17T16:18:01.913816786+08:00 耗时: 4.232288ms  读/写: 2102/536 字节
GET /v HTTP/1.1
User-Agent: blow
Host: www.xyz.com
Content-Type: plain/text; charset=utf-8
Accept: application/json
Accept-Encoding: gzip, deflate


HTTP/1.1 200 OK
Server: Tengine/2.4.0
Date: Wed, 17 Jan 2024 08:18:01 GMT
Content-Type: application/octet-stream
Content-Length: 106
Connection: keep-alive

www.xyz.com 单向SSL通信 test OK, ssl_protocol is NTLSv1.1 (NTLSv1.1 表示国密，其他表示国际)
```

## 双向 TLCP 双证

```sh
[root@jmjs-PC:/home/zys/tlcp]# export TLCP_CERTS=sm2_client_sign.crt,sm2_client_sign.key,sm2_client_enc.crt,sm2_client_enc.key
[root@jmjs-PC:/home/zys/tlcp]# TLCP=1 berf https://192.168.136.114/v -n1 -H Host:www.abc.com
option Conn type: *tlcp.Conn
option TLCP.Version: TLCP
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [DigitalSignature ContentCommitment]
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [KeyEncipherment DataEncipherment KeyAgreement]
option TLCP.HandshakeComplete: true
option TLCP.DidResume: false
option TLCP.CipherSuite: &{ID:57363 Name:ECC_SM4_CBC_SM3 SupportedVersions:[257] Insecure:false}
### 192.168.136.114:44450->192.168.136.114:443 时间: 2024-01-17T16:21:08.328874915+08:00 耗时: 5.439645ms  读/写: 2101/1563 字节
GET /v HTTP/1.1
User-Agent: blow
Host: www.abc.com
Content-Type: plain/text; charset=utf-8
Accept: application/json
Accept-Encoding: gzip, deflate


HTTP/1.1 200 OK
Server: Tengine/2.4.0
Date: Wed, 17 Jan 2024 08:21:08 GMT
Content-Type: application/octet-stream
Content-Length: 106
Connection: keep-alive

www.abc.com 双向SSL通信 test OK, ssl_protocol is NTLSv1.1 (NTLSv1.1 表示国密，其他表示国际)
```

开 Debug 模式

```sh
[root@jmjs-PC:/home/zys/tlcp]# export TLCP_CERTS=sm2_client_sign.crt,sm2_client_sign.key,sm2_client_enc.crt,sm2_client_enc.key
[root@jmjs-PC:/home/zys/tlcp]# TLCP=1 berf https://192.168.136.114/v -n1 -H Host:www.abc.com -pd
[write] Client Hello, len=47, success=true
>>> ClientHello
Random: bytes=65a78e34ba2d993acfe26db842926b8a122c24766a4ae682a6cd5b6c177eb401
Session ID:
Cipher Suites: ECC_SM4_GCM_SM3, ECC_SM4_CBC_SM3,
Compression Methods: [0]
<<<
[read] Server Hello, len=74
>>> ServerHello
Random: bytes=3e65b79b73408ec81f05fdad918ba1163d7e1ccf5866f170444f574e47524400
Session ID: af26908112594b0d4380e7067347fbade07eb613234f9e85077e63ffe9803c65
Cipher Suite: ECC_SM4_CBC_SM3
Compression Method: 0
<<<
[read] Certificate, len=960
>>> Certificates
Cert[0]:
-----BEGIN CERTIFICATE-----
MIIB1TCCAXygAwIBAgIKMAAAAAAAAAAAAzAKBggqgRzPVQGDdTBWMQswCQYDVQQG
EwJDTjEQMA4GA1UECAwHQmVpSmluZzENMAsGA1UECgwEQkpDQTENMAsGA1UECwwE
QkpDQTEXMBUGA1UEAwwOc20yX3Rlc3Rfc3ViY2EwHhcNMjQwMTE0MDU0NDA5WhcN
MzQwMTExMDU0NDA5WjBWMQswCQYDVQQGEwJDTjEQMA4GA1UECAwHQmVpSmluZzEN
MAsGA1UECgwEQkpDQTENMAsGA1UECwwEQkpDQTEXMBUGA1UEAwwOc20yX3NlcnZl
cl9hYmMwWTATBgcqhkjOPQIBBggqgRzPVQGCLQNCAAQkv1y/TidHO0yk8FZ5OzEQ
rZnc6/kzlIG/blaqFM9/RALB/yjSg2RBt1+1iixEtAPaVvwPBGwRGWqqTR5RFsb8
ozIwMDAJBgNVHRMEAjAAMAsGA1UdDwQEAwIGwDAWBgNVHREEDzANggt3d3cuYWJj
LmNvbTAKBggqgRzPVQGDdQNHADBEAiAdCdgX82Fqa3LMHyBJSZdLRd/cDwXMbrWb
7GTCkJKWbgIgZRe7oDobL6DUrWz3xg2AGQHVvpX2aDgOCirKqPYMt+c=
-----END CERTIFICATE-----
Cert[1]:
-----BEGIN CERTIFICATE-----
MIIB1jCCAXygAwIBAgIKMAAAAAAAAAAABDAKBggqgRzPVQGDdTBWMQswCQYDVQQG
EwJDTjEQMA4GA1UECAwHQmVpSmluZzENMAsGA1UECgwEQkpDQTENMAsGA1UECwwE
QkpDQTEXMBUGA1UEAwwOc20yX3Rlc3Rfc3ViY2EwHhcNMjQwMTE0MDU0NDA5WhcN
MzQwMTExMDU0NDA5WjBWMQswCQYDVQQGEwJDTjEQMA4GA1UECAwHQmVpSmluZzEN
MAsGA1UECgwEQkpDQTENMAsGA1UECwwEQkpDQTEXMBUGA1UEAwwOc20yX3NlcnZl
cl9hYmMwWTATBgcqhkjOPQIBBggqgRzPVQGCLQNCAAR95HI5Kz8iLSGIiaOrcCH9
XMjfV860u84Xwk7TRR2TeYhTgKyARZVQMQ7WcqqsjDmcliboHwyYHaW91bLf6kPz
ozIwMDAJBgNVHRMEAjAAMAsGA1UdDwQEAwIDODAWBgNVHREEDzANggt3d3cuYWJj
LmNvbTAKBggqgRzPVQGDdQNIADBFAiBXJqiy7HHlLdNgGlwvXzgm+Ec1IJLxH8Bg
R5UEve+c8AIhAK0rOHfJaCmopHRvMbzmV/oDOkvdVMfSjaz1yZ06G0RV
-----END CERTIFICATE-----
<<<
[read] Server Key Exchange, len=77
[read] Certificate Request, len=546
>>> Certificate Request
Certificate Types: RSA, ECDSA
Certificate Authorities:
Issuer[0]:
CN=rsa_test_root,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
Issuer[1]:
CN=rsa_test_subca,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
Issuer[2]:
CN=ecc_test_root,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
Issuer[3]:
CN=ecc_test_subca,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
Issuer[4]:
CN=sm2_test_root,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
Issuer[5]:
CN=sm2_test_subca,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
<<<
[read] Server Hello Done, len=4
[write] Certificate, len=954, success=true
>>> Certificates
Cert[0]:
-----BEGIN CERTIFICATE-----
MIIB0zCCAXigAwIBAgIKMAAAAAAAAAAABzAKBggqgRzPVQGDdTBWMQswCQYDVQQG
EwJDTjEQMA4GA1UECAwHQmVpSmluZzENMAsGA1UECgwEQkpDQTENMAsGA1UECwwE
QkpDQTEXMBUGA1UEAwwOc20yX3Rlc3Rfc3ViY2EwHhcNMjQwMTE0MDU0NDA5WhcN
MzQwMTExMDU0NDA5WjBSMQswCQYDVQQGEwJDTjEQMA4GA1UECAwHQmVpSmluZzEN
MAsGA1UECgwEQkpDQTENMAsGA1UECwwEQkpDQTETMBEGA1UEAwwKc20yX2NsaWVu
dDBZMBMGByqGSM49AgEGCCqBHM9VAYItA0IABNG+XLyhLd/c33zP4oq9gOIt4YIK
bC+DmcvGL0uDkHW9/Dc8fSdX0L2C7WCgfIc60ApE5b9nDRpONwYl/bD9z4+jMjAw
MAkGA1UdEwQCMAAwCwYDVR0PBAQDAgbAMBYGA1UdEQQPMA2CC3d3dy54eXouY29t
MAoGCCqBHM9VAYN1A0kAMEYCIQCfu7RvIk1CBAVvRalRkMwynjQQcWLDyWyEK7ge
HMtHYwIhAOre8juLRq/Lvzkb3H4Zmn6Gf86aBhTiqbKcAk2/GO18
-----END CERTIFICATE-----
Cert[1]:
-----BEGIN CERTIFICATE-----
MIIB0jCCAXigAwIBAgIKMAAAAAAAAAAACTAKBggqgRzPVQGDdTBWMQswCQYDVQQG
EwJDTjEQMA4GA1UECAwHQmVpSmluZzENMAsGA1UECgwEQkpDQTENMAsGA1UECwwE
QkpDQTEXMBUGA1UEAwwOc20yX3Rlc3Rfc3ViY2EwHhcNMjQwMTE2MDYxMDIxWhcN
MzQwMTEzMDYxMDIxWjBSMQswCQYDVQQGEwJDTjEQMA4GA1UECAwHQmVpSmluZzEN
MAsGA1UECgwEQkpDQTENMAsGA1UECwwEQkpDQTETMBEGA1UEAwwKc20yX2NsaWVu
dDBZMBMGByqGSM49AgEGCCqBHM9VAYItA0IABBZM9ngYLQEBJa69Gx/4kYsvhoca
dasXSR1wEoP06t88GdymtFMbn0T4oTIfuOljSCPaZLVgNFinGgbdzC+ptvCjMjAw
MAkGA1UdEwQCMAAwCwYDVR0PBAQDAgM4MBYGA1UdEQQPMA2CC3d3dy54eXouY29t
MAoGCCqBHM9VAYN1A0gAMEUCIQCvPGO5WJ0W6GQpLPr2lpms4VXAxAd0L0PF0Jw6
NejcEwIgG9+13XWiYlGDwW0rr+h70aCnlNesnex1drQuSmtoocg=
-----END CERTIFICATE-----
<<<
[write] Client Key Exchange, len=162, success=true
[write] Certificate Verify, len=77, success=true
[write] Finished, len=16, success=true
>>> Finished
verify_data: [192 76 213 5 44 174 143 76 82 32 108 53]
<<<
[read] Finished, len=16
>>> Finished
verify_data: [181 37 137 91 38 144 108 172 18 122 46 23]
<<<
option Conn type: *tlcp.Conn
option TLCP.Version: TLCP
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [DigitalSignature ContentCommitment]
option TLCP.Subject: CN=sm2_server_abc,OU=BJCA,O=BJCA,ST=BeiJing,C=CN
option TLCP.KeyUsage: [KeyEncipherment DataEncipherment KeyAgreement]
option TLCP.HandshakeComplete: true
option TLCP.DidResume: false
option TLCP.CipherSuite: &{ID:57363 Name:ECC_SM4_CBC_SM3 SupportedVersions:[257] Insecure:false}
```

## 性能对比

| 项目              | TPS | 
|-----------------|-----|
| 国密双向双证 ｜ 21589  |
| 国密单向双证 ｜ 22052  |
| TLSv1.3 ｜ 38266 |

```sh
[root@jmjs-PC:/home/zys/tlcp]# TLCP_CERTS=sm2_client_sign.crt,sm2_client_sign.key,sm2_client_enc.crt,sm2_client_enc.key TLCP=1 berf https://192.168.136.114/v -H Host:www.abc.com -d30s
Berf  https://192.168.136.114/v for 30s using 100 goroutine(s), 96 GoMaxProcs.

汇总:
  耗时                 30.004s
  总次/RPS    647784 21589.492
    200       647784 21589.492
  平均读写  56.209 36.864 Mbps

统计         Min      Mean    StdDev      Max
  Latency  2.753ms  4.585ms   3.66ms   542.906ms
  RPS      14545.6  21585.61  1364.49  22183.62

百分位延迟:
  P50       P75      P90      P95      P99     P99.9     P99.99
  4.52ms  4.561ms  4.742ms  4.925ms  5.507ms  15.163ms  167.534ms
[root@jmjs-PC:/home/zys/tlcp]# TLCP=1 berf https://192.168.136.114/v -H Host:www.xyz.com -d30s
Berf  https://192.168.136.114/v for 30s using 100 goroutine(s), 96 GoMaxProcs.

汇总:
  耗时                 30.004s
  总次/RPS    661672 22052.419
    200       661672 22052.419
  平均读写  57.388 37.623 Mbps

统计         Min       Mean    StdDev      Max
  Latency   422µs    4.488ms   1.752ms  263.777ms
  RPS      20754.17  22048.21  349.89     22384

百分位延迟:
  P50        P75      P90     P95      P99     P99.9     P99.99
  4.473ms  4.515ms  4.622ms  4.79ms  5.299ms  11.935ms  73.916ms
[root@jmjs-PC:/home/zys/tlcp]#
[root@jmjs-PC:/home/zys/tlcp]#
[root@jmjs-PC:/home/zys/tlcp]# berf https://192.168.136.114/v -H Host:www.xyz.com -d30s
Berf  https://192.168.136.114/v for 30s using 100 goroutine(s), 96 GoMaxProcs.

汇总:
  耗时                 30.002s
  总次/RPS   1148091 38266.588
    200      1148091 38266.588
  平均读写  89.290 54.384 Mbps

统计         Min       Mean    StdDev      Max
  Latency    44µs    2.488ms   1.055ms  129.446ms
  RPS      35884.99  38255.11  1751.37  43394.31

百分位延迟:
  P50       P75      P90      P95      P99    P99.9    P99.99
  2.557ms  2.64ms  3.083ms  3.433ms  3.932ms  7.66ms  47.109ms
[root@jmjs-PC:/home/zys/tlcp]#
```