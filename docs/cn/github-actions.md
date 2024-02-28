# 使用 GitHub Actions

此存储库构建 Docker 镜像并将其部署到 [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) 上
每个批准的拉取请求或设置后在您自己的分支上。

## 设置 GitHub Actions

在存储库设置中的机密下，添加以下机密：

- `REGISTRY_LOGIN_SERVER`: 要使用的 docker 注册表 (例如 `docker.io`).
- `REGISTRY_USERNAME`: 用于登录注册表的用户名 (例如 `dunglas`).
- `REGISTRY_PASSWORD`: 用于登录注册表的密码 (例如 访问密钥).
- `IMAGE_NAME`: 镜像的名称 (例如 `dunglas/frankenphp`).

## 构建和推送镜像

1. 创建拉取请求或推送到分支。
2. GitHub Actions 将生成镜像并运行任何测试。
3. 如果生成成功，则将使用 `pr-x`，其中 `x` 是 PR 编号，作为标记将镜像推送到注册表。

## 部署镜像

1. 合并拉取请求后，GitHub Actions 将再次运行测试并生成新镜像。
2. 如果构建成功，则 Docker 注册表中的 `main` 标记将更新。

## 释放

1. 在存储库中创建新标签。
2. GitHub Actions 将生成镜像并运行任何测试。
3. 如果构建成功，镜像将使用标记名称作为标记推送到注册表(例如，将创建 `v1.2.3` 和 `v1.2`)。
4. `latest` 标签也将更新。
