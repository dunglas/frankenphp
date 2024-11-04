# FrankenPHP: 适用于 PHP 的现代应用服务器

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP 是建立在 [Caddy](https://caddyserver.com/) Web 服务器之上的现代 PHP 应用程序服务器。

FrankenPHP 凭借其令人惊叹的功能为您的 PHP 应用程序提供了超能力：[早期提示](early-hints.md)、[worker 模式](worker.md)、[实时功能](mercure.md)、自动 HTTPS、HTTP/2 和 HTTP/3 支持......

FrankenPHP 可与任何 PHP 应用程序一起使用，并且由于提供了与 worker 模式的集成，使您的 Symfony 和 Laravel 项目比以往任何时候都更快。

FrankenPHP 也可以用作独立的 Go 库，将 PHP 嵌入到任何使用 net/http 的应用程序中。

[**了解更多** *frankenphp.dev*](https://frankenphp.dev/cn) 以及在以下地址中：

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## 开始

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

访问 `https://localhost`, 并享受吧!

> [!TIP]
>
> 不要尝试使用 `https://127.0.0.1`。使用 `https://localhost` 并接受自签名证书。
> 使用 [`SERVER_NAME` 环境变量](config.md#环境变量) 更改要使用的域。

### 独立二进制

如果您不想使用 Docker，我们为 Linux 和 macOS 提供独立的 FrankenPHP 二进制文件
，其中包含 [PHP 8.3](https://www.php.net/releases/8.3/en.php) 和最流行的 PHP 扩展：[下载 FrankenPHP](https://github.com/dunglas/frankenphp/releases)。

若要启动当前目录的内容，请运行：

```console
./frankenphp php-server
```

您还可以使用以下命令运行命令行脚本：

```console
./frankenphp php-cli /path/to/your/script.php
```

## 文档

* [worker 模式](worker.md)
* [早期提示支持(103 HTTP status code)](early-hints.md)
* [实时功能](mercure.md)
* [配置](config.md)
* [Docker 镜像](docker.md)
* [在生产环境中部署](production.md)
* [创建独立、可自行执行的 PHP 应用程序](embed.md)
* [创建静态二进制文件](static.md)
* [从源代码编译](compile.md)
* [Laravel 集成](laravel.md)
* [已知问题](known-issues.md)
* [演示应用程序 (Symfony) 和性能测试](https://github.com/dunglas/frankenphp-demo)
* [Go 库文档](https://pkg.go.dev/github.com/dunglas/frankenphp)
* [贡献和调试](https://frankenphp.dev/docs/contributing/)

## 示例和框架

* [Symfony](https://github.com/dunglas/symfony-docker)
* [API Platform](https://api-platform.com/docs/distribution/)
* [Laravel](laravel.md)
* [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
* [WordPress](https://github.com/StephenMiracle/frankenwp)
* [Drupal](https://github.com/dunglas/frankenphp-drupal)
* [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
* [TYPO3](https://github.com/ochorocho/franken-typo3)
