# 已知问题

## Fibers

在 [Fibers](https://www.php.net/manual/en/language.fibers.php) 中调用 PHP 的函数和代码等语言结构，这些结构内部再调用 [cgo](https://go.dev/blog/cgo) 会导致崩溃。

这个问题 [正在由 Go 项目处理](https://github.com/golang/go/issues/62130)。

一种解决方案是不要使用从 Fibers 内部委托给 Go 的构造（如 `echo`）和函数（如 `header()`）。

下面的代码可能会崩溃，因为它在 Fiber 中使用了 `echo`：

```php
$fiber = new Fiber(function() {
    echo 'In the Fiber'.PHP_EOL;
    echo 'Still inside'.PHP_EOL;
});
$fiber->start();
```

相反，请从 Fiber 返回值并在外部使用它：

```php
$fiber = new Fiber(function() {
    Fiber::suspend('In the Fiber'.PHP_EOL));
    Fiber::suspend('Still inside'.PHP_EOL));
});
echo $fiber->start();
echo $fiber->resume();
$fiber->resume();
```

## 不支持的 PHP 扩展

已知以下扩展与 FrankenPHP 不兼容：

| 名称                                                          | 原因    | 替代方案                                                                                                                 |
|-------------------------------------------------------------|-------|----------------------------------------------------------------------------------------------------------------------|
| [imap](https://www.php.net/manual/en/imap.installation.php) | 非线程安全 | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |

## get_browser

[get_browser()](https://www.php.net/manual/en/function.get-browser.php) 函数在一段时间后似乎表现不佳。解决方法是缓存（例如使用 [APCu](https://www.php.net/manual/zh/book.apcu.php)）每个 User-Agent，因为它们是不变的。

## 独立的二进制和基于 Alpine 的 Docker 镜像

独立的二进制文件和基于 Alpine 的 docker 镜像 (`dunglas/frankenphp:*-alpine`) 使用的是 [musl libc](https://musl.libc.org/) 而不是 [glibc and friends](https://www.etalabs.net/compare_libcs.html)，为的是保持较小的二进制大小。
这可能会导致一些兼容性问题。特别是，glob 标志 `GLOB_BRACE` [不可用](https://www.php.net/manual/en/function.glob.php)。

## 在 Docker 中使用 `https://127.0.0.1`

默认情况下，FrankenPHP 会为 `localhost` 生成一个 TLS 证书。
这是本地开发最简单且推荐的选项。

如果确实想使用 `127.0.0.1` 作为主机，可以通过将服务器名称设置为 `127.0.0.1` 来配置它以为其生成证书。

如果你使用 Docker，因为 [Docker 网络](https://docs.docker.com/network/) 问题，只做这些是不够的。
您将收到类似于以下内容的 TLS 错误 `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`。

如果你使用的是 Linux，解决方案是使用 [使用宿主机网络](https://docs.docker.com/network/network-tutorial-host/)：

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Mac 和 Windows 不支持 Docker 使用宿主机网络。在这些平台上，您必须猜测容器的 IP 地址并将其包含在服务器名称中。

运行 `docker network inspect bridge` 并查看 `Containers`，找到 `IPv4Address` 当前分配的最后一个 IP 地址，并增加 1。如果没有容器正在运行，则第一个分配的 IP 地址通常为 `172.17.0.2`。

然后将其包含在 `SERVER_NAME` 环境变量中：

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> 请务必将 `172.17.0.3` 替换为将分配给容器的 IP。

您现在应该能够从主机访问 `https://127.0.0.1`。

如果不是这种情况，请在调试模式下启动 FrankenPHP 以尝试找出问题：

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
