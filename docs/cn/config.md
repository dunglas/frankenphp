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

可以在站点块中使用 `php_server` 或 `php` [HTTP 指令](https://caddyserver.com/docs/caddyfile/concepts#directives) 来为您的 PHP 应用程序提供服务。

最小示例：

```caddyfile
localhost {
	# 启用压缩（可选）
	encode zstd br gzip
	# 执行当前目录中的 PHP 文件并提供资产
	php_server
}
```

您也可以使用全局选项显式配置 FrankenPHP：
`frankenphp` [全局选项](https://caddyserver.com/docs/caddyfile/concepts#global-options) 可用于配置 FrankenPHP。

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

如果在同一服务器上运行多个应用，还可以定义多个 worker：

```caddyfile
app.example.com {
	php_server {
		root /path/to/app/public
		worker index.php <num>
	}
}

other.example.com {
	php_server {
		root /path/to/other/public
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
	root <directory> # 设置站点的根目录。默认值：`root` 指令。
	split_path <delim...> # 设置用于将 URI 拆分为两部分的子字符串。第一个匹配的子字符串将用于从路径中拆分"路径信息"。第一个部分以匹配的子字符串为后缀，并将假定为实际资源(CGI 脚本)名称。第二部分将设置为PATH_INFO，供脚本使用。默认值：`.php`
	resolve_root_symlink false # 禁用将 `root` 目录在符号链接时将其解析为实际值（默认启用）。
	env <key> <value> # 设置额外的环境变量，可以设置多个环境变量。
	file_server off # 禁用内置的 file_server 指令。
	worker { # 创建特定于此服务器的 worker。可以为多个 worker 多次指定。
		file <path> # 设置 worker 脚本的路径，可以相对于 php_server 根目录
		num <num> # 设置要启动的 PHP 线程数，默认为可用 CPU 数的 2 倍
		name <name> # 为 worker 设置名称，用于日志和指标。默认值：worker 文件的绝对路径。在 php_server 块中定义时始终以 m# 开头。
		watch <path> # 设置要监视文件更改的路径。可以为多个路径多次指定。
		env <key> <value> # 将额外的环境变量设置为给定值。可以为多个环境变量多次指定。此 worker 的环境变量也从 php_server 父级继承，但可以在此处覆盖。
	}
	worker <other_file> <num> # 也可以像在全局 frankenphp 块中一样使用简短形式。
}
```

## 环境变量

以下环境变量可用于在 `Caddyfile` 中注入 Caddy 指令，而无需对其进行修改：

- `SERVER_NAME`: 更改 [要监听的地址](https://caddyserver.com/docs/caddyfile/concepts#addresses)，提供的主机名也将用于生成的 TLS 证书
- `CADDY_GLOBAL_OPTIONS`: 注入 [全局选项](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: 在 `frankenphp` 指令下注入配置

## PHP 配置

要加载 [其他 PHP INI 配置文件](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan)，
可以使用 `PHP_INI_SCAN_DIR` 环境变量。
设置后，PHP 将加载给定目录中存在 `.ini` 扩展名的所有文件。

## 启用调试模式

使用 Docker 镜像时，将 `CADDY_GLOBAL_OPTIONS` 环境变量设置为 `debug` 以启用调试模式：

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
