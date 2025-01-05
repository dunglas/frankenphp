# Создание кастомных Docker-образов

[Docker-образы FrankenPHP](https://hub.docker.com/r/dunglas/frankenphp) основаны на [официальных PHP-образах](https://hub.docker.com/_/php/). Доступны варианты для Debian и Alpine Linux для популярных архитектур. Рекомендуется использовать версии Debian.

Доступны версии для PHP 8.2, 8.3 и 8.4.

Теги следуют следующему шаблону: `dunglas/frankenphp:<frankenphp-version>-php<php-version>-<os>`.

* `<frankenphp-version>` и `<php-version>` — версии FrankenPHP и PHP соответственно: от основных (например, `1`) до минорных (например, `1.2`) и патч-версий (например, `1.2.3`).
* `<os>` может быть `bookworm` (для Debian Bookworm) или `alpine` (для последней стабильной версии Alpine).

[Просмотреть доступные теги](https://hub.docker.com/r/dunglas/frankenphp/tags).

## Как использовать образы

Создайте `Dockerfile` в вашем проекте:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Затем выполните следующие команды для сборки и запуска Docker-образа:

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## Как установить дополнительные PHP-расширения

Скрипт [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) включён в базовый образ. Установка дополнительных PHP-расширений осуществляется просто:

```dockerfile
FROM dunglas/frankenphp

# Добавьте дополнительные расширения здесь:
RUN install-php-extensions \
	pdo_mysql \
	gd \
	intl \
	zip \
	opcache
```

## Как установить дополнительные модули Caddy

FrankenPHP построен на базе Caddy, и все [модули Caddy](https://caddyserver.com/docs/modules/) можно использовать с FrankenPHP.

Самый простой способ установить пользовательские модули Caddy — использовать [xcaddy](https://github.com/caddyserver/xcaddy):

```dockerfile
FROM dunglas/frankenphp:builder AS builder

# Копируем xcaddy в образ сборки
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# Для сборки FrankenPHP необходимо включить CGO
RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp=./ \
        --with github.com/dunglas/frankenphp/caddy=./caddy/ \
        --with github.com/dunglas/caddy-cbrotli \
        # Mercure и Vulcain включены в официальный билд, но вы можете их удалить
        --with github.com/dunglas/mercure/caddy \
        --with github.com/dunglas/vulcain/caddy
        # Добавьте дополнительные модули Caddy здесь

FROM dunglas/frankenphp AS runner

# Заменяем официальный бинарник на пользовательский с добавленными модулями
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

> [!TIP]
>
> Если вы используете Alpine Linux и Symfony, возможно, потребуется [увеличить размер стека](compile.md#использование-xcaddy).

## Активировать Worker режим по умолчанию

Установите переменную окружения `FRANKENPHP_CONFIG`, чтобы запускать FrankenPHP с Worker-скриптом:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Использование тома в разработке

Для удобной разработки с FrankenPHP смонтируйте директорию с исходным кодом приложения на хосте как том в Docker-контейнере:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> Опция `--tty` позволяет видеть удобочитаемые логи вместо JSON-формата.

С использованием Docker Compose:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    tty: true

volumes:
  caddy_data:
  caddy_config:
```

## Запуск под обычным пользователем

FrankenPHP поддерживает запуск под обычным пользователем в Docker.

Пример `Dockerfile` для этого:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	# Для Alpine используйте "adduser -D ${USER}"
	useradd ${USER}; \
	setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
	chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy

USER ${USER}
```

### Запуск без дополнительных прав

Если вы используете непривилегированные порты (от 1024 и выше), сервер можно запускать без root-прав:

```dockerfile
FROM dunglas/frankenphp

ARG USER=appuser

RUN \
	useradd ${USER}; \
	setcap -r /usr/local/bin/frankenphp; \
	chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy

USER ${USER}
```

## Обновления

Docker-образы обновляются:

* при выпуске новой версии;
* ежедневно в 4 утра UTC, если доступны новые версии официальных PHP-образов.

## Версии для разработки

Версии для разработки доступны в [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev).  
Новая сборка запускается при каждом коммите в ветку `main`.