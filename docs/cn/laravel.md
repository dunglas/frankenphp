# Laravel

## Docker

使用 FrankenPHP 为 [Laravel](https://laravel.com) Web 应用程序提供服务就像将项目挂载到官方 Docker 镜像的 `/app` 目录中一样简单。

从 Laravel 应用程序的主目录运行以下命令：

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

尽情享受吧！

## 本地安装

或者，你可以从本地机器上使用 FrankenPHP 运行 Laravel 项目：

1. [下载与您的系统相对应的二进制文件](https://github.com/dunglas/frankenphp/releases)
2. 将以下配置添加到 Laravel 项目根目录中名为 `Caddyfile` 的文件中：

    ```caddyfile
    {
    	frankenphp
    }

    # 服务器的域名
    localhost {
    	# 将 webroot 设置为 public/ 目录
    	root * public/
    	# 启用压缩(可选)
    	encode zstd br gzip
    	# 执行当前目录中的 PHP 文件并提供资产
    	php_server
    }
    ```

3. 从 Laravel 项目的根目录启动 FrankenPHP：`frankenphp run`

## Laravel Octane

Octane 可以通过 Composer 包管理器安装：

```console
composer require laravel/octane
```

安装 Octane 后，您可以执行 `octane:install` Artisan 命令，该命令会将 Octane 的配置文件安装到您的应用程序中：

```console
php artisan octane:install --server=frankenphp
```

Octane 服务可以通过 `octane:frankenphp` Artisan 命令启动。

```console
php artisan octane:frankenphp
```

`octane:frankenphp` 命令可以采用以下选项：

* `--host`: 服务器应绑定到的 IP 地址（默认值: `127.0.0.1`）
* `--port`: 服务器应可用的端口（默认值: `8000`）
* `--admin-port`: 管理服务器应可用的端口（默认值: `2019`）
* `--workers`: 应可用于处理请求的 worker 数（默认值: `auto`）
* `--max-requests`: 在 worker 重启之前要处理的请求数（默认值: `500`）
* `--caddyfile`: FrankenPHP `Caddyfile` 文件的路径
* `--https`: 开启 HTTPS、HTTP/2 和 HTTP/3，自动生成和延长证书
* `--http-redirect`: 启用 HTTP 到 HTTPS 重定向（仅在使用 `--https` 时启用）
* `--watch`: 修改应用程序时自动重新加载服务器
* `--poll`: 在监视时使用文件系统轮询，以便通过网络监视文件
* `--log-level`: 在指定日志级别或高于指定日志级别的日志消息

你可以了解更多关于 [Laravel Octane 官方文档](https://laravel.com/docs/octane)。
