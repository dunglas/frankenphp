# Конфигурация

FrankenPHP, Caddy, а также модули Mercure и Vulcain могут быть настроены с использованием [конфигурационных форматов, поддерживаемых Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

В [Docker-образах](docker.md) файл `Caddyfile` находится по пути `/etc/caddy/Caddyfile`.
Статический бинарный файл будет искать `Caddyfile` в директории запуска.

PHP можно настроить [с помощью файла `php.ini`](https://www.php.net/manual/en/configuration.file.php).

PHP из Docker-образов и статического бинарного файла по умолчанию будет искать файл `php.ini` в директории, где был запущен FrankenPHP, а также в `/usr/local/etc/php/`. Кроме того, будут загружены все файлы с расширением `.ini` из директории `/usr/local/etc/php/conf.d/`.

Файл `php.ini` по умолчанию отсутствует. Вы можете скопировать официальный шаблон, предоставляемый проектом PHP.

В Docker-шаблоны включены в образы:

```dockerfile
FROM dunglas/frankenphp

# Для production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Или для development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

Если вы не используете Docker, скопируйте один из шаблонов `php.ini-production` или `php.ini-development`, предоставленных [в исходниках PHP](https://github.com/php/php-src/).

## Конфигурация Caddyfile

Для настройки FrankenPHP установите [глобальную опцию](https://caddyserver.com/docs/caddyfile/concepts#global-options) frankenphp. После этого можно использовать [HTTP-директивы](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` или `php` для обработки вашего PHP-приложения.

Минимальный пример:

```caddyfile
{
	# Включить FrankenPHP
	frankenphp
}

localhost {
	# Включить сжатие (опционально)
	encode zstd br gzip
	# Выполнять PHP-файлы в текущей директории и обслуживать ресурсы
	php_server
}
```

Опционально можно указать количество потоков и [worker-скриптов](worker.md), которые запускаются вместе с сервером:

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

Вы также можете определить несколько workers, если обслуживаете несколько приложений на одном сервере:

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker /path/to/other/public/index.php <num>
	}
}

app.example.com {
	root /path/to/app/public
	php_server
}

other.example.com {
	root /path/to/other/public
	php_server
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

Для загрузки [дополнительных конфигурационных файлов PHP](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan) можно использовать переменную окружения `PHP_INI_SCAN_DIR`.
Если она установлена, PHP загрузит все файлы с расширением `.ini`, находящиеся в указанных директориях.

## Включение режима отладки

При использовании Docker-образа установите переменную окружения `CADDY_GLOBAL_OPTIONS` в `debug`, чтобы включить режим отладки:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
