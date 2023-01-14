variable "REPO_NAME" {
    default = "dunglas/frankenphp"
}

group "default" {
    targets = ["bullseye-variants", "alpine-variants"]
}

group "bullseye-variants" {
    targets = ["bullseye-php-82"]
}

group "alpine-variants" {
    targets = ["alpine-php-82"]
}

target "common" {
    context = "."
    platforms = ["linux/amd64", "linux/arm64"]
}

target "common-bullseye" {
    contexts = {
        php-base = "docker-image://php:8.2-zts-bullseye"
        golang-base = "docker-image://golang:1.19-bullseye"
    }
}

target "common-alpine" {
    contexts = {
        php-base = "docker-image://php:8.2-zts-alpine3.17"
        golang-base = "docker-image://golang:1.19-alpine3.17"
    }
}

# Builders

target "builder-bullseye" {
    inherits = ["common-bullseye"]
    dockerfile = "builder-bullseye.Dockerfile"
}

target "builder-alpine" {
    inherits = ["common-alpine"]
    dockerfile = "builder-alpine.Dockerfile"
}

#
# FrankenPHP
#

target "bullseye-php-82" {
    inherits = ["common", "common-bullseye"]
    contexts = {
        builder = "target:builder-bullseye"
    }
    tags = ["${REPO_NAME}:bullseye", "${REPO_NAME}:latest"]
}

target "alpine-php-82" {
    inherits = ["common", "common-alpine"]
    contexts = {
        builder = "target:builder-alpine"
    }
    tags = ["${REPO_NAME}:alpine"]
}
