# 构建自定义 Docker 镜像

[FrankenPHP Docker 镜像](https://hub.docker.com/r/dunglas/frankenphp) 基于 [官方 PHP 镜像](https://hub.docker.com/_/php/)。
Alpine Linux 和 Debian 衍生版适用于常见的处理器架构，且支持 PHP 8.2 和 PHP 8.3。[查看 Tags](https://hub.docker.com/r/dunglas/frankenphp/tags)。

## 如何使用镜像

在项目中创建 `Dockerfile`：

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

然后运行以下命令以构建并运行 Docker 镜像：

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## 如何安装更多 PHP 扩展

[`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) 脚本在基础镜像中提供。
添加额外的 PHP 扩展很简单：

```dockerfile
FROM dunglas/frankenphp

# 在此处添加其他扩展：
RUN install-php-extensions \
  pdo_mysql \
  gd \
  intl \
  zip \
  opcache
```

## 如何安装更多 Caddy 模块

FrankenPHP 建立在 Caddy 之上，所有 [Caddy 模块](https://caddyserver.com/docs/modules/) 都可以与 FrankenPHP 一起使用。

安装自定义 Caddy 模块的最简单方法是使用 [xcaddy](https://github.com/caddyserver/xcaddy)：

```dockerfile
FROM dunglas/frankenphp:latest-builder AS builder

# 在构建器镜像中复制 xcaddy
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# 必须启用 CGO 才能构建 FrankenPHP
ENV CGO_ENABLED=1 XCADDY_SETCAP=1 XCADDY_GO_BUILD_FLAGS="-ldflags '-w -s'"
RUN xcaddy build \
  --output /usr/local/bin/frankenphp \
  --with github.com/dunglas/frankenphp=./ \
  --with github.com/dunglas/frankenphp/caddy=./caddy/ \
  --with github.com/dunglas/caddy-cbrotli \
  # Mercure 和 Vulcain 包含在官方版本中，如果不需要你可以删除它们
  --with github.com/dunglas/mercure/caddy \
  --with github.com/dunglas/vulcain/caddy
  # 在此处添加额外的 Caddy 模块

FROM dunglas/frankenphp AS runner

# 将官方二进制文件替换为包含自定义模块的二进制文件
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

FrankenPHP 提供的 `builder` 镜像包含 libphp 的编译版本。
[用于构建的镜像](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) 适用于所有版本的 FrankenPHP 和 PHP，包括 Alpine 和 Debian。

> [!TIP]
>
> 如果你的系统基于 musl libc（Alpine Linux 上默认使用）并搭配 Symfony 使用，
> 您可能需要 [增加默认堆栈大小](compile.md#使用-xcaddy)。

## 默认启用 worker 模式

设置 `FRANKENPHP_CONFIG` 环境变量以使用 worker 脚本启动 FrankenPHP：

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## 开发挂载宿主机目录

要使用 FrankenPHP 轻松开发，请从包含应用程序源代码的主机挂载目录作为 Docker 容器中的 volume：

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 -p 443:443/udp --tty my-php-app
```

> [!TIP]
>
> `--tty` 选项允许使用清晰可读的日志，而不是 JSON 日志。

使用 Docker Compose：

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # 如果要使用自定义 Dockerfile，请取消注释以下行
    #build: .
    # 如果要在生产环境中运行，请取消注释以下行
    # restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # 在生产环境中注释以下行，它允许在 dev 中使用清晰可读日志
    tty: true

# Caddy 证书和配置所需的挂载目录
volumes:
  caddy_data:
  caddy_config:
```

## 以非 root 用户身份运行

FrankenPHP 可以在 Docker 中以非 root 用户身份运行。

下面是一个示例 Dockerfile：

```dockerfile
FROM dunglas/frankenphp

ARG USER=www-data

RUN \
  # 在基于 alpine 的发行版使用 "adduser -D ${USER}"
  useradd -D ${USER}; \
  # 需要开放80和443端口的权限
  setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp; \
  # 需要 /data/caddy 和 /config/caddy 目录的写入权限
  chown -R ${USER}:${USER} /data/caddy && chown -R ${USER}:${USER} /config/caddy;

USER ${USER}
```

## 更新

Docker 镜像会按照以下条件更新：

* 发布新的版本后
* 每日 4:00（UTC 时间）检查新的 PHP 镜像

## 开发版本

可在此 [`dunglas/frankenphp-dev`](https://hub.docker.com/repository/docker/dunglas/frankenphp-dev) 仓库获取开发版本。
每次在 GitHub 仓库的主分支有新的 commit 都会触发一次新的 build。

`latest*` tag 指向最新的 `main` 分支，且同样支持 `sha-<git-commit-hash>` 的 tag。
