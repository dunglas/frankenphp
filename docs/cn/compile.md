# 从源代码编译

本文档解释了如何创建一个 FrankenPHP 构建，它将 PHP 加载为一个动态库。
这是推荐的方法。

或者，你也可以 [编译静态版本](static.md)。

## 安装 PHP

FrankenPHP 支持 PHP 8.2 及更高版本。

首先，[获取 PHP 源代码](https://www.php.net/downloads.php) 并提取它们：

```console
tar xf php-*
cd php-*/
```

然后，为您的平台配置 PHP.

这些参数是必需的，但你也可以添加其他编译参数（例如额外的扩展）。

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

### Mac

使用 [Homebrew](https://brew.sh/) 包管理器安装 `libiconv`、`bison`、`re2c` 和 `pkg-config`：

```console
brew install libiconv bison re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

然后运行 `./configure` 脚本：

```console
./configure \
    --enable-embed=static \
    --enable-zts \
    --disable-zend-signals \
    --disable-opcache-jit \
    --enable-static \
    --enable-shared=no \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

## 编译并安装 PHP

最后，编译并安装 PHP：

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## 安装可选依赖项

FrankenPHP 的某些功能需要安装可选依赖项。
这些功能也可以通过向 Go 编译器传递编译标签来禁用。

| 功能                                                  | 依赖项                                                              | 禁用的编译标签 |
| ----------------------------------------------------- | ------------------------------------------------------------------- | -------------- |
| Brotli 压缩                                           | [Brotli](https://github.com/google/brotli)                          | nobrotli       |
| 文件更改时重启工作进程                                | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher      |

## 编译 Go 应用

您现在可以使用 Go 库并编译我们的 Caddy 构建：

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main
./install_dependencies.sh
cd caddy/frankenphp
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
go build -tags=nobadger,nomysql,nopgx
```

请注意，这将生成一个没有 Mercure 或 Vulcain 的 `frankenphp` 二进制文件。对于生产用途，最好使用 xcaddy。

### 使用 xcaddy

你可以使用 [xcaddy](https://github.com/caddyserver/xcaddy) 来编译 [自定义 Caddy 模块](https://caddyserver.com/docs/modules/) 的 FrankenPHP：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Add extra Caddy modules here
```

> [!TIP]
>
> 如果你的系统基于 musl libc（Alpine Linux 上默认使用）并搭配 Symfony 使用，
> 您可能需要增加默认堆栈大小。
> 否则，您可能会收到如下错误 `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> 请将 `XCADDY_GO_BUILD_FLAGS` 环境变量更改为如下类似的值
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> （根据您的应用需求更改堆栈大小）。
> 有关更多信息，请查看 build-static.sh 文件。
