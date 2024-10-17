# 使用 FrankenPHP Workers

启动应用程序一次并将其保存在内存中。
FrankenPHP 将在几毫秒内处理传入的请求。

## 启动 Worker 脚本

### Docker

将 `FRANKENPHP_CONFIG` 环境变量的值设置为 `worker /path/to/your/worker/script.php`：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 独立二进制

使用 `php-server` 命令的 `--worker` 选项， 执行命令使当前目录的内容使用 worker：

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

## Symfony Runtime

FrankenPHP 的 worker 模式由 [Symfony Runtime 组件](https://symfony.com/doc/current/components/runtime.html) 支持。
要在 worker 中启动任何 Symfony 应用程序，请安装 [PHP Runtime](https://github.com/php-runtime/runtime) 的 FrankenPHP 软件包：

```console
composer require runtime/frankenphp-symfony
```

通过定义 `APP_RUNTIME` 环境变量来启动你的应用服务器，以使用 FrankenPHP Symfony Runtime：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

请参阅 [文档](laravel.md#laravel-octane)。

## 自定义应用程序

以下示例演示如何在不依赖第三方库的情况下创建自己的 worker 脚本：

```php
<?php
// public/index.php

// 防止在客户端连接中断时 worker 线程脚本终止
ignore_user_abort(true);

// 启动应用
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// 循环外的处理程序以获得更好的性能（减少工作量）
$handler = static function () use ($myApp) {
    // 收到请求时调用
    // 超全局变量 php://input
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};
for ($nbRequests = 0, $running = true; isset($_SERVER['MAX_REQUESTS']) && ($nbRequests < ((int)$_SERVER['MAX_REQUESTS'])) && $running; ++$nbRequests) {
    $running = \frankenphp_handle_request($handler);

    // 发送 HTTP 响应后执行某些操作
    $myApp->terminate();

    // 调用垃圾回收器以减少在页面生成过程中触发垃圾回收器的几率
    gc_collect_cycles();
}
// 结束清理
$myApp->shutdown();
```

然后，启动应用并使用 `FRANKENPHP_CONFIG` 环境变量来配置你的 worker：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

默认情况下，每个 CPU 启动一个 worker。
您还可以配置要启动的 worker 数：

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### 在一定数量的请求后重新启动 Worker

由于 PHP 最初不是为长时间运行的进程而设计的，因此仍然有许多库和遗留代码会发生内存泄露。
在 worker 模式下使用此类代码的解决方法是在处理一定数量的请求后重新启动 worker 程序脚本：

前面的 worker 代码段允许通过设置名为 `MAX_REQUESTS` 的环境变量来配置要处理的最大请求数。
