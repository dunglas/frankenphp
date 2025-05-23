# 配置

FrankenPHP，Caddy 以及 Mercure 和 Vulcain 模块可以使用 [Caddy 支持的格式](https://caddyserver.com/docs/getting-started#your-first-config) 进行配置。

在[Docker 映像](docker.md) 中，`Caddyfile` 位于 `/etc/frankenphp/Caddyfile`。
静态二进制文件会在启动时所在的目录中查找 `Caddyfile`。
PHP 本身可以[使用 `php.ini` 文件](https://www.php.net/manual/zh/configuration.file.php)进行配置。
PHP 解释器将在以下位置查找：

Docker:

- php.ini: `/usr/local/etc/php/php.ini` 默认情况下不提供 php.ini。
- 附加配置文件: `/usr/local/etc/php/conf.d/*.ini`
- php 扩展: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- 您应该复制 PHP 项目提供的官方模板：

```dockerfile
FROM dunglas/frankenphp

# 生产:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# 开发:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

FrankenPHP 安装 (.rpm 或 .deb):

- php.ini: `/etc/frankenphp/php.ini` 默认情况下提供带有生产预设的 php.ini 文件。
- 附加配置文件: `/etc/frankenphp/php.d/*.ini`
- php 扩展: `/usr/lib/frankenphp/modules/`

静态二进制:

- php.ini: 执行 `frankenphp run` 或 `frankenphp php-server` 的目录，然后是 `/etc/frankenphp/php.ini`
- 附加配置文件: `/etc/frankenphp/php.d/*.ini`
- php 扩展: 无法加载
- 复制[PHP 源代码](https://github.com/php/php-src/)中提供的`php.ini-production`或`php.ini-development`中的一个。

## Caddyfile 配置

要注册 FrankenPHP 执行器，必须设置 `frankenphp` [全局选项](https://caddyserver.com/docs/caddyfile/concepts#global-options)，然后可以在站点块中使用 `php_server` 或 `php` [HTTP 指令](https://caddyserver.com/docs/caddyfile/concepts#directives) 来为您的 PHP 应用程序提供服务。

最小示例：

```caddyfile
{
	# 启用 FrankenPHP
	frankenphp
}

localhost {
	# 启用压缩（可选）
	encode zstd br gzip
	# 执行当前目录中的 PHP 文件并提供资产
	php_server
}
```

或者，可以在全局选项下指定要创建的线程数和要从服务器启动的 [worker 脚本](worker.md)。

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # 设置要启动的 PHP 线程数。默认值：可用 CPU 数量的 2 倍。
		worker {
			file <path> # 设置 worker 脚本的路径。
			num <num> # 设置要启动的 PHP 线程数，默认为可用 CPU 数的 2 倍。
			env <key> <value> # 将额外的环境变量设置为给定值。可以为多个环境变量多次指定。
		}
	}
}

# ...
```

或者，您可以使用 `worker` 选项的一行缩写形式：

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Worker 块也可以在 `php` 或 `php_server` 块内定义。在这种情况下，worker 继承父指令的环境变量和根路径，并且只能由该特定域访问：

```caddyfile
{
	frankenphp
}
example.com {
	root /path/to/app
	php_server {
		root <path>
		worker {
			file <path, 可以相对于 root>
			num <num>
			env <key> <value>
			watch <path>
			name <name>
		}
	}
}
```

如果在同一服务器上运行多个应用，还可以定义多个 worker：

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker {
			file /path/to/other/public/index.php
			num <num>
			env APP_ENV dev
		}
	}
}

app.example.com {
	root /path/to/app/public
	php_server
}

other.example.com {
	root /path/to/other/public
	php_server {
		env APP_ENV dev
	}
}

# ...
```

等效于

```caddyfile
{
	frankenphp
}

app.example.com {
	php_server {
		root /path/to/app/public
		worker index.php <num>
	}
}

other.example.com {
	php_server {
		root /path/to/other/public
		env APP_ENV dev
		worker index.php <num>
	}
}
# ...
```

通常你只需要 `php_server` 指令，
但如果要完全控制，则可以使用较低级别的 `php` 指令：

使用 `php_server` 指令等效于以下配置：

```caddyfile
route {
	# 为目录请求添加尾部斜杠
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# 如果请求的文件不存在，则尝试 index 文件
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

`php_server` 和 `php` 指令具有以下选项：

```caddyfile
php_server [<matcher>] {
	root <directory> # 设置站点的根目录。默认值：`root` 指令。如果未指定
	split_path <delim...> # 设置用于将 URI 拆分为两部分的子字符串。第一个匹配的子字符串将用于从路径中拆分“路径信息”。第一个部分以匹配的子字符串为后缀，并将假定为实际资源(CGI 脚本)名称。第二部分将设置为PATH_INFO，供脚本使用。默认值：`.php`
	resolve_root_symlink false # 禁用将 `root` 目录在符号链接时将其解析为实际值（默认启用）。
	env <key> <value> # 设置额外的环境变量，可以设置多个环境变量。
}
```

## 环境变量

以下环境变量可用于在 `Caddyfile` 中注入 Caddy 指令，而无需对其进行修改：

- `SERVER_NAME`: 更改 [要监听的地址](https://caddyserver.com/docs/caddyfile/concepts#addresses)，提供的主机名也将用于生成的 TLS 证书
- `CADDY_GLOBAL_OPTIONS`: 注入 [全局选项](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: 在 `frankenphp` 指令下注入配置

## PHP 配置

您还可以使用 `frankenphp` 块中的 `php_ini` 指令更改 PHP 配置：

```caddyfile
{
	frankenphp {
		php_ini memory_limit 256M

		# 或者

		php_ini {
			memory_limit 256M
			max_execution_time 15
		}
	}
}
```

要加载 [其他 PHP INI 配置文件](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan)，
可以使用 `PHP_INI_SCAN_DIR` 环境变量。
设置后，PHP 将加载给定目录中存在 `.ini` 扩展名的所有文件。

## php-server 命令

`php-server` 命令是启动生产就绪 PHP 服务器的便捷方式。它特别适用于快速部署、演示、开发或运行[嵌入式应用](embed.md)。

```console
frankenphp php-server [--domain <example.com>] [--root <path>] [--listen <addr>] [--worker /path/to/worker.php<,nb-workers>] [--watch <paths...>] [--access-log] [--debug] [--no-compress] [--mercure]
```

### 选项

- `--domain`, `-d`: 提供文件的域名。如果指定，服务器将使用 HTTPS 并自动获取 Let's Encrypt 证书。
- `--root`, `-r`: 站点根目录的路径。如果未指定并使用嵌入式应用，默认将使用 embedded_app/public 目录。
- `--listen`, `-l`: 绑定监听器的地址。默认为 `:80`，如果指定了域名则为 `:443`。
- `--worker`, `-w`: 要运行的 worker 脚本。可以多次指定以运行多个 worker。
- `--watch`: 监视文件更改的目录。可以多次指定以监视多个目录。
- `--access-log`, `-a`: 启用访问日志。
- `--debug`, `-v`: 启用详细调试日志。
- `--mercure`, `-m`: 启用内置的 Mercure.rocks hub。
- `--no-compress`: 禁用 Zstandard、Brotli 和 Gzip 压缩。

### 示例

使用当前目录作为文档根目录启动服务器：

```console
frankenphp php-server --root ./
```

启动启用 HTTPS 的服务器：

```console
frankenphp php-server --domain example.com
```

启动带有 worker 的服务器：

```console
frankenphp php-server --worker public/index.php
```

## 启用调试模式

使用 Docker 镜像时，将 `CADDY_GLOBAL_OPTIONS` 环境变量设置为 `debug` 以启用调试模式：

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
