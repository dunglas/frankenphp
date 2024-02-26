# 创建静态构建

由于伟大的 [static-php-cli 项目](https://github.com/crazywhalecc/static-php-cli)，创建 FrankenPHP 的静态构建是可能的(尽管它的名字，这个项目支持所有 SAPI，而不仅仅是 CLI)，
而不是使用 PHP 库的本地安装。

使用这种方法，一个可移植的二进制文件将包含 PHP 解释器、Caddy Web 服务器和 FrankenPHP！

FrankenPHP 还支持 [将 PHP 应用程序嵌入到静态二进制文件中](embed.md)。

## Linux

我们提供了一个 Docker 镜像来构建 Linux 静态二进制文件：

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

生成的静态二进制文件名为 `frankenphp` ，可在当前目录中找到。

如果您想在没有 Docker 的情况下构建静态二进制文件，请查看 macOS 说明，它也适用于 Linux。

### 自定义扩展

默认情况下，大多数流行的 PHP 扩展都会被编译。

若要减小二进制文件的大小并减少攻击面，可以选择使用 `PHP_EXTENSIONS` Docker ARG 构建的扩展列表。

例如，运行以下命令以仅生成 `opcache` 扩展：

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

若要将启用其他功能的库添加到已启用的扩展中，可以使用 `PHP_EXTENSION_LIBS` Docker ARG：

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

参见：[自定义构建](#自定义构建)

### GitHub Token

如果达到 GitHub API 速率限制，请在名为 `GITHUB_TOKEN` 的环境变量中设置 GitHub Personal Access Token：

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

运行以下脚本以创建适用于 macOS 的静态二进制文件(必须安装 [Homebrew](https://brew.sh/))：

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

注意：此脚本也适用于 Linux(可能也适用于其他 Unix)，并由我们提供的基于 Docker 的静态构建器在内部使用。

## 自定义构建

以下环境变量可以传递给 `docker build` 和 `build-static.sh`
脚本来自定义静态构建：

* `FRANKENPHP_VERSION`: 要使用的 FrankenPHP 版本
* `PHP_VERSION`: 要使用的 PHP 版本
* `PHP_EXTENSIONS`: 要构建的 PHP 扩展 ([支持的扩展列表](https://static-php.dev/en/guide/extensions.html))
* `PHP_EXTENSION_LIBS`: 要构建的额外库，为扩展添加额外的功能
* `EMBED`: 要嵌入二进制文件的 PHP 应用程序的路径
* `CLEAN`: 设置后，libphp 及其所有依赖项都是从头开始构建的(无缓存)
* `DEBUG_SYMBOLS`: 设置后，调试符号不会被剥离，而是将添加到二进制文件中
* `RELEASE`: (仅限维护者)设置后，生成的二进制文件将上传到 GitHub 上
