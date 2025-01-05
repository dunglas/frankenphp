# Worker режим в FrankenPHP

Загрузите приложение один раз и держите его в памяти.  
FrankenPHP обрабатывает входящие запросы за несколько миллисекунд.

## Запуск Worker-скриптов

### Docker

Установите значение переменной окружения `FRANKENPHP_CONFIG` на `worker /path/to/your/worker/script.php`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Standalone бинарник

Используйте опцию `--worker` команды `php-server`, чтобы обслуживать содержимое текущей директории через Worker-скрипт:

```console
frankenphp php-server --worker /path/to/your/worker/script.php
```

Если ваше PHP-приложение [встроено в бинарник](embed.md), вы можете добавить пользовательский `Caddyfile` в корневую директорию приложения.  
Он будет использоваться автоматически.

Также можно настроить [автоматический перезапуск Worker-скрипта при изменении файлов](config.md#отслеживание-изменений-файлов) с помощью опции `--watch`.  
Следующая команда выполнит перезапуск, если будет изменён любой файл с расширением `.php` в директории `/path/to/your/app/` или её поддиректориях:

```console
frankenphp php-server --worker /path/to/your/worker/script.php --watch "/path/to/your/app/**/*.php"
```

## Symfony Runtime

Worker режим FrankenPHP поддерживается компонентом [Symfony Runtime](https://symfony.com/doc/current/components/runtime.html).  
Чтобы запустить любое Symfony-приложение в Worker режиме, установите пакет FrankenPHP для [PHP Runtime](https://github.com/php-runtime/runtime):

```console
composer require runtime/frankenphp-symfony
```

Запустите сервер приложения, задав переменную окружения `APP_RUNTIME` для использования FrankenPHP Symfony Runtime:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Подробнее см. в [документации](laravel.md#laravel-octane).

## Пользовательские приложения

Следующий пример показывает, как создать собственный Worker-скрипт без использования сторонних библиотек:

```php
<?php
// public/index.php

// Предотвращает завершение Worker-скрипта при разрыве соединения клиента
ignore_user_abort(true);

// Инициализация приложения
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Обработчик запросов за пределами цикла для повышения производительности
$handler = static function () use ($myApp) {
    // Выполняется при обработке запроса.
    // Суперглобальные переменные, php://input и другие данные обновляются для каждого запроса.
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);

    // Действия после отправки HTTP-ответа
    $myApp->terminate();

    // Вызов сборщика мусора, чтобы снизить вероятность его запуска в процессе генерации страницы
    gc_collect_cycles();

    if (!$keepRunning) break;
}

// Завершение
$myApp->shutdown();
```

Запустите приложение, настроив Worker-скрипт с помощью переменной окружения `FRANKENPHP_CONFIG`:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Настройка количества Worker-скриптов

По умолчанию запускается по 2 Worker-скрипта на каждый CPU.  
Вы можете задать своё значение:

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Перезапуск Worker-скрипта после определённого количества запросов

PHP изначально не был разработан для долгоживущих процессов, поэтому некоторые библиотеки и устаревший код могут приводить к утечкам памяти.  
Для решения этой проблемы можно настроить автоматический перезапуск Worker-скрипта после обработки определённого количества запросов.  

В предыдущем примере максимальное количество запросов задаётся с помощью переменной окружения `MAX_REQUESTS`.

### Сбои Worker-скрипта

Если Worker-скрипт завершится с ненулевым кодом выхода, FrankenPHP перезапустит его с использованием экспоненциальной задержки.  
Если Worker-скрипт проработает дольше, чем время последней задержки * 2, он будет считаться стабильным, и задержка сбросится.  
Однако, если Worker-скрипт продолжает завершаться с ненулевым кодом выхода в течение короткого промежутка времени (например, из-за опечатки в коде), FrankenPHP завершит работу с ошибкой: `too many consecutive failures`.

## Поведение суперглобальных переменных

[PHP суперглобальные переменные](https://www.php.net/manual/en/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET` и т.д.) ведут себя следующим образом:

* до первого вызова `frankenphp_handle_request()` суперглобальные переменные содержат значения, связанные с самим Worker-скриптом
* во время и после вызова `frankenphp_handle_request()` суперглобальные переменные содержат значения, сгенерированные на основе обработанного HTTP-запроса, каждый вызов изменяет значения суперглобальных переменных

Чтобы получить доступ к суперглобальным переменным Worker-скрипта внутри колбэка, необходимо скопировать их и импортировать копию в область видимости колбэка:

```php
<?php
// Копирование $_SERVER Worker-скрипта перед первым вызовом frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER для запроса
    var_dump($workerServer); // $_SERVER Worker-скрипта
};

// ...
```