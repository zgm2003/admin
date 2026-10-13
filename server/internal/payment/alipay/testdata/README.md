# 支付宝公共证书格式夹具

`alipayRootCert.crt` 与 `alipayPublicCert.crt` 来自锁定的
`github.com/go-pay/gopay v1.5.118/alipay/cert` 公共 SDK 样本。没有商户私钥或凭据。

根包包含不同算法与历史签名；公钥文件包含叶证书和中间证书。测试用于证明完整文件格式
与 SDK RSA2 SN 选择兼容；历史叶证书已过期，不用于证明当前可支付或证书信任链认证。
