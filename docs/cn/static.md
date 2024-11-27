# 创建静态构建

基于 [static-php-cli](https://github.com/crazywhalecc/static-php-cli) 项目（这个项目支持所有 SAPI，不仅仅是 `cli`），
FrankenPHP 已支持创建静态二进制，无需安装本地 PHP。

使用这种方法，我们可构建一个包含 PHP 解释器、Caddy Web 服务器和 FrankenPHP 的可移植二进制文件！

FrankenPHP 还支持 [将 PHP 应用程序嵌入到静态二进制文件中](embed.md)。

## Linux

我们提供了一个 Docker 镜像来构建 Linux 静态二进制文件：

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

生成的静态二进制文件名为 `frankenphp`，可在当前目录中找到。

如果您想在没有 Docker 的情况下构建静态二进制文件，请查看 macOS 说明，它也适用于 Linux。

### 自定义扩展

默认情况下，大多数流行的 PHP 扩展都会被编译。

若要减小二进制文件的大小并减少攻击面，可以选择使用 `PHP_EXTENSIONS` Docker 参数来自定义构建的扩展。

例如，运行以下命令以生成仅包含 `opcache,pdo_sqlite` 扩展的二进制：

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

若要将启用其他功能的库添加到已启用的扩展中，可以使用 `PHP_EXTENSION_LIBS` Docker 参数：

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

### 额外的 Caddy 模块

要向 [xcaddy](https://github.com/caddyserver/xcaddy) 添加额外的 Caddy 模块或传递其他参数，请使用 `XCADDY_ARGS` Docker 参数：

```console
docker buildx bake \
  --load \
  --set static-builder.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder
```

在本例中，我们为 Caddy 添加了 [Souin](https://souin.io) HTTP 缓存模块，以及 [Mercure](https://mercure.rocks) 和 [Vulcain](https://vulcain.rocks) 模块。

> [!TIP]
>
> 如果 `XCADDY_ARGS` 为空或未设置，则默认包含 Mercure 和 Vulcain 模块。
> 如果自定义了 `XCADDY_ARGS` 的值，则必须显式地包含它们。

参见：[自定义构建](#自定义构建)

### GitHub Token

如果遇到了 GitHub API 速率限制，请在 `GITHUB_TOKEN` 的环境变量中设置 GitHub Personal Access Token：

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

运行以下脚本以创建适用于 macOS 的静态二进制文件（需要先安装 [Homebrew](https://brew.sh/)）：

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

注意：此脚本也适用于 Linux（可能也适用于其他 Unix 系统），我们提供的用于构建静态二进制的 Docker 镜像也在内部使用这个脚本。

## 自定义构建

以下环境变量可以传递给 `docker build` 和 `build-static.sh`
脚本来自定义静态构建：

* `FRANKENPHP_VERSION`: 要使用的 FrankenPHP 版本
* `PHP_VERSION`: 要使用的 PHP 版本
* `PHP_EXTENSIONS`: 要构建的 PHP 扩展（[支持的扩展列表](https://static-php.dev/zh/guide/extensions.html)）
* `PHP_EXTENSION_LIBS`: 要构建的额外库，为扩展添加额外的功能
* `XCADDY_ARGS`：传递给 [xcaddy](https://github.com/caddyserver/xcaddy) 的参数，例如用于添加额外的 Caddy 模块
* `EMBED`: 要嵌入二进制文件的 PHP 应用程序的路径
* `CLEAN`: 设置后，libphp 及其所有依赖项都是重新构建的（不使用缓存）
* `DEBUG_SYMBOLS`: 设置后，调试符号将被保留在二进制文件内
* `RELEASE`: （仅限维护者）设置后，生成的二进制文件将上传到 GitHub 上
