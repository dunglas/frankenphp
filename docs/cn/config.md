# 配置

FrankenPHP，Caddy 以及 Mercure 和 Vulcain 模块可以使用 [Caddy 支持的格式](https://caddyserver.com/docs/getting-started#your-first-config) 进行配置。

在 Docker 镜像中，`Caddyfile` 位于 `/etc/caddy/Caddyfile`。

您也可以像往常一样使用 `php.ini` 配置 PHP。

在 Docker 镜像中，`php.ini` 文件不存在，您可以手动创建它或 `复制`。

```dockerfile
FROM dunglas/frankenphp

# 开发:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini

# 还是生产:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
```

## Caddyfile 配置

要注册 FrankenPHP 执行器，必须设置 `frankenphp` [全局选项](https://caddyserver.com/docs/caddyfile/concepts#global-options)，然后可以在站点块中使用 `php_server` 或 `php` [HTTP 指令](https://caddyserver.com/docs/caddyfile/concepts#directives)来为您的 PHP 应用程序提供服务。

极小示例：

```caddyfile
{
	# 启用 FrankenPHP
	frankenphp
	# 配置何时必须执行指令
	order php_server before file_server
}

localhost {
	# 启用压缩(可选)
	encode zstd br gzip
	# 执行当前目录中的 PHP 文件并提供资产
	php_server
}
```

或者，可以在全局选项下指定要创建的线程数和要从服务器启动的 [worker scripts](worker.md)。

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

如果在同一服务器上提供多个应用，还可以定义多个 worker：

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker /path/to/other/public/index.php <num>
	}
}

app.example.com {
	root * /path/to/app/public
	php_server
}

other.example.com {
	root * /path/to/other/public
	php_server
}
...
```

使用 `php_server` 指令通常是您需要的，
但是，如果您需要完全控制，则可以使用较低级别的 `php` 指令：

使用 `php_server` 指令等效于以下配置：

```caddyfile
route {
	# 为目录请求添加尾部斜杠
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# 如果请求的文件不存在，请尝试 index 文件
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
	root <directory> # 设置站点的根文件夹。默认值：`root` 指令。
	split_path <delim...> # 设置用于将 URI 拆分为两部分的子字符串。第一个匹配的子字符串将用于从路径中拆分“路径信息”。第一个部分以匹配的子字符串为后缀，并将假定为实际资源(CGI 脚本)名称。第二部分将设置为PATH_INFO，供 CGI 脚本使用。默认值：`.php`
	resolve_root_symlink # 允许通过计算符号链接(如果存在)将 `根` 目录解析为其实际值。
	env <key> <value> # 将额外的环境变量设置为给定值。可以为多个环境变量多次指定。
}
```

## 环境变量

以下环境变量可用于在 `Caddyfile` 中注入 Caddy 指令，而无需对其进行修改：

* `SERVER_NAME`: 更改 [要监听的地址](https://caddyserver.com/docs/caddyfile/concepts#addresses)，提供的主机名也将用于生成的 TLS 证书
* `CADDY_GLOBAL_OPTIONS`: 注入 [全局选项](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG`: 在 `frankenphp` 指令下注入配置

与 FPM 和 CLI SAPIs 不同，默认情况下，环境变量不会在超全局变量 `$_SERVER` 和 `$_ENV` 中公开。

要将环境变量传播到 `$_SERVER` 和 `$_ENV`，请将 `php.ini` `variables_order` 指令设置为 `EGPCS`。

## PHP 配置

要加载[其他PHP配置文件](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan)，
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
