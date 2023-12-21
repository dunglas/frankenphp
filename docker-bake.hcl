variable "IMAGE_NAME" {
    default = "dunglas/frankenphp"
}

variable "VERSION" {
    default = "dev"
}

variable "PHP_VERSION" {
    default = "8.2,8.3"
}

variable "GO_VERSION" {
    default = "1.21"
}

variable "SHA" {}

variable "LATEST" {
    default = false
}

variable "CACHE" {
    default = ""
}

variable DEFAULT_PHP_VERSION {
    default = "8.3"
}

function "tag" {
    params = [version, os, php-version, tgt]
    result = [
        version != "" ? format("%s:%s%s-php%s-%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", php-version, os) : "",
        php-version == DEFAULT_PHP_VERSION && os == "bookworm"  && version != "" ? format("%s:%s%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "") : "",
        php-version == DEFAULT_PHP_VERSION && version != "" ? format("%s:%s%s-%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", os) : "",
        php-version == DEFAULT_PHP_VERSION && version == "latest" ? format("%s:%s%s", IMAGE_NAME, os, tgt == "builder" ? "-builder" : "") : "",
        os == "bookworm" && version != "" ? format("%s:%s%s-php%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", php-version) : "",
    ]
}

# cleanTag ensures that the tag is a valid Docker tag
# see https://github.com/distribution/distribution/blob/v2.8.2/reference/regexp.go#L37
function "clean_tag" {
    params = [tag]
    result = substr(regex_replace(regex_replace(tag, "[^\\w.-]", "-"), "^([^\\w])", "r$0"), 0, 127)
}

# semver adds semver-compliant tag if a semver version number is passed, or returns the revision itself
# see https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
function "semver" {
  params = [rev]
  result = __semver(_semver(regexall("^v?(?P<major>0|[1-9]\\d*)\\.(?P<minor>0|[1-9]\\d*)\\.(?P<patch>0|[1-9]\\d*)(?:-(?P<prerelease>(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$", rev)))
}

function "_semver" {
    params = [matches]
    result = length(matches) == 0 ? {} : matches[0]
}

function "__semver" {
    params = [v]
    result = v == {} ? [clean_tag(VERSION)] : v.prerelease == null ? ["latest", v.major, "${v.major}.${v.minor}", "${v.major}.${v.minor}.${v.patch}"] : ["${v.major}.${v.minor}.${v.patch}-${v.prerelease}"]
}

function "php_version" {
    params = [v]
    result = _php_version(v, regexall("(?P<major>\\d+)\\.(?P<minor>\\d+)", v)[0])
}

function "_php_version" {
    params = [v, m]
    result = "${m.major}.${m.minor}" == DEFAULT_PHP_VERSION ? [v, "${m.major}.${m.minor}", "${m.major}"] : [v, "${m.major}.${m.minor}"]
}

target "default" {
    name = "${tgt}-php-${replace(php-version, ".", "-")}-${os}"
    matrix = {
        os = ["bookworm", "alpine"]
        php-version = split(",", PHP_VERSION)
        tgt = ["builder", "runner"]
    }
    contexts = {
        php-base = "docker-image://php:${php-version}-zts-${os}"
        golang-base = "docker-image://golang:${GO_VERSION}-${os}"
    }
    dockerfile = os == "alpine" ? "alpine.Dockerfile" : "Dockerfile"
    context = "./"
    target = tgt
    # arm/v6 is only available for Alpine: https://github.com/docker-library/golang/issues/502
    platforms = os == "alpine" ? [
        "linux/amd64",
        "linux/386",
        "linux/arm/v6",
        "linux/arm/v7",
        "linux/arm64",
    ] : [
        "linux/amd64",
        "linux/386",
        "linux/arm/v7",
        "linux/arm64"
    ]
    tags = distinct(flatten(
        [for pv in php_version(php-version) : flatten([
            LATEST ? tag("latest", os, pv, tgt) : [],
            tag(SHA == "" ? "" : "sha-${substr(SHA, 0, 7)}", os, pv, tgt),
            [for v in semver(VERSION) : tag(v, os, pv, tgt)]
        ])
    ]))
    labels = {
        "org.opencontainers.image.created" = "${timestamp()}"
        "org.opencontainers.image.version" = VERSION
        "org.opencontainers.image.revision" = SHA
    }
    args = {
        FRANKENPHP_VERSION = VERSION
    }
}

target "static-builder" {
    contexts = {
        golang-base = "docker-image://golang:${GO_VERSION}-alpine"
    }
    dockerfile = "static-builder.Dockerfile"
    context = "./"
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ]
    tags = distinct(flatten([
        LATEST ? "${IMAGE_NAME}:static-builder" : "",
        SHA == "" ? "" : "${IMAGE_NAME}:static-builder-sha-${substr(SHA, 0, 7)}",
        [for v in semver(VERSION) : v == "latest" ? "${IMAGE_NAME}:static-builder": "${IMAGE_NAME}:static-builder-${v}"]
    ]))
    labels = {
        "org.opencontainers.image.created" = "${timestamp()}"
        "org.opencontainers.image.version" = VERSION
        "org.opencontainers.image.revision" = SHA
    }
    args = {
        FRANKENPHP_VERSION = VERSION
    }
    secret = ["id=github-token,env=GITHUB_TOKEN"]
}
