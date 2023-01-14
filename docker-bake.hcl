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

#
# FrankenPHP
#

target "bullseye-php-82" {
    inherits = ["common"]
    contexts = {
        php-base = "docker-image://php:8.2-zts-bullseye"
        golang-base = "docker-image://golang:1.19-bullseye"
    }
    dockerfile = "Dockerfile"
    tags = ["${REPO_NAME}:bullseye", "${REPO_NAME}:latest"]
}

target "alpine-php-82" {
    inherits = ["common"]
    contexts = {
        php-base = "docker-image://php:8.2-zts-alpine3.17"
        golang-base = "docker-image://golang:1.19-alpine3.17"
    }
    dockerfile = "Dockerfile.alpine"
    tags = ["${REPO_NAME}:alpine"]
}
