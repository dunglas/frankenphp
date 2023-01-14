variable "REPO_NAME" {
    default = "dunglas/frankenphp"
}

group "default" {
    targets = ["bookworm-variants", "alpine-variants"]
}

group "bookworm-variants" {
    targets = ["bookworm-php-82", "builder-bookworm-php-82"]
}

group "alpine-variants" {
    targets = ["alpine-php-82", "builder-alpine-php-82"]
}

target "common" {
    context = "."
    platforms = ["linux/amd64", "linux/arm64"]
}

target "common-bookworm" {
    contexts = {
        php-base = "docker-image://php:8.2-zts-bookworm"
        golang-base = "docker-image://golang:1.20-bookworm"
    }
}

target "common-alpine" {
    contexts = {
        php-base = "docker-image://php:8.2-zts-alpine3.18"
        golang-base = "docker-image://golang:1.20-alpine3.18"
    }
}

# Builders

target "builder-bookworm-php-82" {
    inherits = ["common-bookworm"]
    dockerfile = "builder-debian.Dockerfile"
    tags = ["${REPO_NAME}:builder", "${REPO_NAME}:builder-bookworm"]
}

target "builder-alpine-php-82" {
    inherits = ["common-alpine"]
    dockerfile = "builder-alpine.Dockerfile"
    tags = ["${REPO_NAME}:builder-alpine"]
}

#
# FrankenPHP
#

target "bookworm-php-82" {
    inherits = ["common", "common-bookworm"]
    contexts = {
        builder = "target:builder-bookworm-php-82"
    }
    tags = ["${REPO_NAME}:bookworm", "${REPO_NAME}:latest"]
}

target "alpine-php-82" {
    inherits = ["common", "common-alpine"]
    contexts = {
        builder = "target:builder-alpine-php-82"
    }
    tags = ["${REPO_NAME}:alpine"]
}
