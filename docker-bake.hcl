variable "IMAGE_NAME" {
    default = "dunglas/frankenphp"
}

variable "VERSION" {
    default = "dev"
}

variable "SHA" {}

variable "LATEST" {
    default = false
}

variable "CACHE" {
    default = ""
}

function "tag" {
    params = [version, os, php-version, tgt]
    result = [
        version != "" ? format("%s:%s%s-php%s-%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", php-version, os) : "",
        os == "bookworm" && php-version == "8.2" && version != "" ? format("%s:%s%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "") : "",
        php-version == "8.2" && version != "" ? format("%s:%s%s-%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", os) : "",
        os == "bookworm" && version != "" ? format("%s:%s%s-php%s", IMAGE_NAME, version, tgt == "builder" ? "-builder" : "", php-version) : ""
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

target "default" {
    name = "${tgt}-php-${replace(php-version, ".", "-")}-${os}"
    matrix = {
        os = ["bookworm", "alpine"]
        php-version = ["8.2", "8.3.0beta3"]
        tgt = ["builder", "runner"]
    }
    contexts = {
        php-base = "docker-image://php:${php-version}-zts-${os}"
        golang-base = "docker-image://golang:1.20-${os}"
    }
    dockerfile = os == "alpine" ? "alpine.Dockerfile" : "Dockerfile"
    context = "./"
    target = tgt
    platforms = [
        "linux/amd64",
        "linux/386",
        "linux/arm/v6",
        "linux/arm/v7",
        "linux/arm64",
    ]
    tags = distinct(flatten([
        LATEST ? tag("latest", os, php-version, tgt) : [],
        tag(SHA == "" ? "" : "sha-${substr(SHA, 0, 7)}", os, php-version, tgt),
        [for v in semver(VERSION) : tag(v, os, php-version, tgt)]
    ]))
    labels = {
        "org.opencontainers.image.title" = "FrankenPHP"
        "org.opencontainers.image.description" = "The modern PHP app server"
        "org.opencontainers.image.url" = "https://frankenphp.dev"
        "org.opencontainers.image.source" = "https://github.com/dunglas/frankenphp"
        "org.opencontainers.image.licenses" = "MIT"
        "org.opencontainers.image.vendor" = "Kévin Dunglas"
        "org.opencontainers.image.created" = "${timestamp()}"
        "org.opencontainers.image.version" = VERSION
        "org.opencontainers.image.revision" = SHA
    }
    args = {
        FRANKENPHP_VERSION = VERSION
    }
}
