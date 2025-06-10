# Конфигурация

FrankenPHP, Caddy, а также модули Mercure и Vulcain могут быть настроены с использованием [конфигурационных форматов, поддерживаемых Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

В [Docker-образах](docker.md) файл `Caddyfile` находится по пути `/etc/frankenphp/Caddyfile`.
Статический бинарный файл будет искать `Caddyfile` в директории запуска.

PHP можно настроить [с помощью файла `php.ini`](https://www.php.net/manual/en/configuration.file.php).

PHP-интерпретатор будет искать в следующих местах:

Docker:

- php.ini: `/usr/local/etc/php/php.ini` По умолчанию php.ini не предоставляется.
- дополнительные файлы конфигурации: `/usr/local/etc/php/conf.d/*.ini`
- расширения php: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- Вы должны скопировать официальный шаблон, предоставляемый проектом PHP:

```dockerfile
FROM dunglas/frankenphp

# Для production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Или для development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

Установка FrankenPHP (.rpm или .deb):

- php.ini: `/etc/frankenphp/php.ini` По умолчанию предоставляется файл php.ini с производственными настройками.
- дополнительные файлы конфигурации: `/etc/frankenphp/php.d/*.ini`
- расширения php: `/usr/lib/frankenphp/modules/`

Статический бинарный файл:

- php.ini: Директория, в которой выполняется `frankenphp run` или `frankenphp php-server`, затем `/etc/frankenphp/php.ini`
- дополнительные файлы конфигурации: `/etc/frankenphp/php.d/*.ini`
- расширения php: не могут быть загружены
- скопируйте один из шаблонов `php.ini-production` или `php.ini-development`, предоставленных [в исходниках PHP](https://github.com/php/php-src/).

## Конфигурация Caddyfile

[HTTP-директивы](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` или `php` могут быть использованы в блоках сайта для обработки вашего PHP-приложения.

Минимальный пример:

```caddyfile
localhost {
	# Включить сжатие (опционально)
	encode zstd br gzip
	# Выполнять PHP-файлы в текущей директории и обслуживать ресурсы
	php_server
}
```

Вы также можете явно настроить FrankenPHP с помощью глобальной опции:
[Глобальная опция](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp` может быть использована для настройки FrankenPHP.

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Указывает количество потоков PHP. По умолчанию: 2x от числа доступных CPU.
		worker {
			file <path> # Указывает путь к worker-скрипту.
			num <num> # Указывает количество потоков PHP. По умолчанию: 2x от числа доступных CPU.
			env <key> <value> # Устанавливает дополнительную переменную окружения. Можно указать несколько раз для разных переменных.
			watch <path> # Указывает путь для отслеживания изменений файлов.Можно указать несколько раз для разных путей.
		}
	}
}

# ...
```

В качестве альтернативы можно использовать однострочную краткую форму для опции `worker`:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Блоки worker также могут быть определены внутри блока `php` или `php_server`. В этом случае worker наследует переменные окружения и корневой путь от родительской директивы и доступен только для этого конкретного домена:

```caddyfile
{
	frankenphp
}
example.com {
	root /path/to/app
	php_server {
		root <path>
		worker {
			file <path, может быть относительным к root>
			num <num>
			env <key> <value>
			watch <path>
			name <name>
		}
	}
}
```

Вы также можете определить несколько workers, если обслуживаете несколько приложений на одном сервере:

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

Использование директивы `php_server` — это то, что нужно в большинстве случаев. Однако если требуется полный контроль, вы можете использовать более низкоуровневую директиву `php`:

Использование директивы `php_server` эквивалентно следующей конфигурации:

```caddyfile
route {
	# Добавить слэш в конец запросов к директориям
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# Если запрошенный файл не существует, попытаться использовать файлы index
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

Директивы `php_server` и `php` имеют следующие опции:

```caddyfile
php_server [<matcher>] {
	root <directory> # Указывает корневую директорию сайта. По умолчанию: директива `root`.
	split_path <delim...> # Устанавливает подстроки для разделения URI на две части. Первая часть будет использована как имя ресурса (CGI-скрипта), вторая часть — как PATH_INFO. По умолчанию: `.php`.
	resolve_root_symlink false # Отключает разрешение символьных ссылок для `root` (включено по умолчанию).
	env <key> <value> # Устанавливает дополнительные переменные окружения. Можно указать несколько раз для разных переменных.
	file_server off # Отключает встроенную директиву file_server.
	worker { # Создает worker, специфичный для этого сервера. Можно указать несколько раз для разных workers.
		file <path> # Указывает путь к worker-скрипту, может быть относительным к корню php_server
		num <num> # Указывает количество потоков PHP. По умолчанию: 2x от числа доступных CPU.
		name <name> # Устанавливает имя для worker, используемое в логах и метриках. По умолчанию: абсолютный путь к файлу worker. Всегда начинается с m# при определении в блоке php_server.
		watch <path> # Указывает путь для отслеживания изменений файлов. Можно указать несколько раз для разных путей.
		env <key> <value> # Устанавливает дополнительную переменную окружения. Можно указать несколько раз для разных переменных. Переменные окружения для этого worker также наследуются от родительского php_server, но могут быть переопределены здесь.
	}
	worker <other_file> <num> # Также можно использовать краткую форму как в глобальном блоке frankenphp.
}
```

### Отслеживание изменений файлов

Поскольку workers запускают ваше приложение только один раз и держат его в памяти, изменения в PHP-файлах не будут применяться сразу.

Для разработки можно настроить перезапуск workers при изменении файлов с помощью директивы `watch`:

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

Если директория для `watch` не указана, по умолчанию будет использоваться путь `./**/*.{php,yaml,yml,twig,env}`,
который отслеживает все файлы с расширениями `.php`, `.yaml`, `.yml`, `.twig` и `.env` в директории, где был запущен процесс FrankenPHP, и во всех её поддиректориях. Вы также можете указать одну или несколько директорий с использованием [шаблона имён файлов](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # отслеживает все файлы во всех поддиректориях /path/to/app
			watch /path/to/app/*.php # отслеживает файлы с расширением .php в /path/to/app
			watch /path/to/app/**/*.php # отслеживает PHP-файлы в /path/to/app и поддиректориях
			watch /path/to/app/**/*.{php,twig} # отслеживает PHP и Twig-файлы в /path/to/app и поддиректориях
		}
	}
}
```

- Шаблон `**` указывает на рекурсивное отслеживание.
- Директории могут быть указаны относительно директории запуска FrankenPHP.
- Если у вас определено несколько workers, все они будут перезапущены при изменении файлов.
- Избегайте отслеживания файлов, создаваемых во время выполнения (например, логов), так как это может вызвать нежелательные перезапуски.

Механизм отслеживания файлов основан на [e-dant/watcher](https://github.com/e-dant/watcher).

### Полный дуплекс (HTTP/1)

При использовании HTTP/1.x можно включить режим полного дуплекса, чтобы разрешить запись ответа до завершения чтения тела запроса (например, для WebSocket, Server-Sent Events и т.д.).

Эта опция включается вручную и должна быть добавлена в глобальные настройки `Caddyfile`:

```caddyfile
{
	servers {
		enable_full_duplex
	}
}
```

> [!CAUTION]
>
> Включение этой опции может привести к зависанию устаревших HTTP/1.x клиентов, которые не поддерживают полный дуплекс.
> Настройка также доступна через переменную окружения `CADDY_GLOBAL_OPTIONS`:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

Дополнительную информацию об этой настройке можно найти в [документации Caddy](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Переменные окружения

Следующие переменные окружения могут быть использованы для добавления директив в `Caddyfile` без его изменения:

- `SERVER_NAME`: изменение [адресов для прослушивания](https://caddyserver.com/docs/caddyfile/concepts#addresses); предоставленные хостнеймы также будут использованы для генерации TLS-сертификата.
- `CADDY_GLOBAL_OPTIONS`: добавление [глобальных опций](https://caddyserver.com/docs/caddyfile/options).
- `FRANKENPHP_CONFIG`: добавление конфигурации в директиву `frankenphp`.

Как и для FPM и CLI SAPIs, переменные окружения по умолчанию доступны в суперглобальной переменной `$_SERVER`.

Значение `S` в [директиве PHP `variables_order`](https://www.php.net/manual/en/ini.core.php#ini.variables-order) всегда эквивалентно `ES`, независимо от того, где расположена `E` в этой директиве.

## Конфигурация PHP

Вы также можете изменить конфигурацию PHP, используя директиву `php_ini` в блоке `frankenphp`:

```caddyfile
{
	frankenphp {
		php_ini memory_limit 256M

		# или

		php_ini {
			memory_limit 256M
			max_execution_time 15
		}
	}
}
```

Для загрузки [дополнительных конфигурационных файлов PHP](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan) можно использовать переменную окружения `PHP_INI_SCAN_DIR`.
Если она установлена, PHP загрузит все файлы с расширением `.ini`, находящиеся в указанных директориях.

## Команда php-server

Команда `php-server` — это удобный способ запустить готовый к продакшну PHP-сервер. Она особенно полезна для быстрого развертывания, демонстраций, разработки или запуска [встроенного приложения](embed.md).

```console
frankenphp php-server [--domain <example.com>] [--root <path>] [--listen <addr>] [--worker /path/to/worker.php<,nb-workers>] [--watch <paths...>] [--access-log] [--debug] [--no-compress] [--mercure]
```

### Опции

- `--domain`, `-d`: Доменное имя, на котором будут обслуживаться файлы. Если указано, сервер будет использовать HTTPS и автоматически получит сертификат Let's Encrypt.
- `--root`, `-r`: Путь к корневой директории сайта. Если не указано и используется встроенное приложение, по умолчанию будет использоваться директория embedded_app/public.
- `--listen`, `-l`: Адрес, к которому будет привязан слушатель. По умолчанию `:80` или `:443`, если указан домен.
- `--worker`, `-w`: Скрипт worker для запуска. Может быть указан несколько раз для нескольких workers.
- `--watch`: Директория для отслеживания изменений файлов. Может быть указана несколько раз для нескольких директорий.
- `--access-log`, `-a`: Включить журнал доступа.
- `--debug`, `-v`: Включить подробные журналы отладки.
- `--mercure`, `-m`: Включить встроенный хаб Mercure.rocks.
- `--no-compress`: Отключить сжатие Zstandard, Brotli и Gzip.

### Примеры

Запуск сервера с текущей директорией в качестве корневой:

```console
frankenphp php-server --root ./
```

Запуск сервера с включенным HTTPS:

```console
frankenphp php-server --domain example.com
```

Запуск сервера с worker:

```console
frankenphp php-server --worker public/index.php
```

## Включение режима отладки

При использовании Docker-образа установите переменную окружения `CADDY_GLOBAL_OPTIONS` в `debug`, чтобы включить режим отладки:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
