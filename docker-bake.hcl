variable "REPO_NAME" {
    default = "dunglas/frankenphp"
}

group "default" {
    targets = ["bullseye", "buster", "alpine315", "alpine316"]
}

group "bullseye" {
    targets = ["bullseye-php-82"]
}

group "buster" {
    targets = ["buster-php-82"]
}

group "alpine315" {
    targets = ["alpine315-php-82"]
}

group "alpine316" {
    targets = ["alpine316-php-82"]
}

target "common" {
    platforms = ["linux/amd64", "linux/arm64"]
    context = "."
    target = "frankenphp"
}

#
# PHP
#

target "php-82" {
    args = {
        PHP_VERSION = "8.2.0RC4"
    }
}

#
# FrankenPHP
#

target "bullseye-php-82" {
    inherits = ["common", "php-82"]
    args = {
        DISTRO = "bullseye"
    }
    tags = ["${REPO_NAME}:bullseye", "${REPO_NAME}:latest"]
}

target "buster-php-82" {
    inherits = ["common", "php-82"]
    args = {
        DISTRO = "buster"
    }
    tags = ["${REPO_NAME}:buster"]
}

target "alpine315-php-82" {
    inherits = ["common", "php-82"]
    args = {
        DISTRO = "alpine315"
    }
    tags = ["${REPO_NAME}:alpine3.15"]
}

target "alpine316-php-82" {
    inherits = ["common", "php-82"]
    args = {
        DISTRO = "alpine316"
    }
    tags = ["${REPO_NAME}:alpine3.16", "${REPO_NAME}:alpine"]
}
