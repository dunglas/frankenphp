variable "REPO_NAME" {
    default = "dunglas/frankenphp"
}

group "default" {
    targets = ["bookworm", "alpine"]
}

target "common" {
    platforms = ["linux/amd64", "linux/arm64"]
}

#
# FrankenPHP
#

target "bookworm" {
    inherits = ["common"]
    context = "."
    dockerfile = "Dockerfile"
    tags = ["${REPO_NAME}:bookworm", "${REPO_NAME}:latest"]
}

target "alpine" {
    inherits = ["common"]
    context = "."
    dockerfile = "Dockerfile.alpine"
    tags = ["${REPO_NAME}:alpine"]
}
