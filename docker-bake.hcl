variable "REPO_NAME" {
    default = "dunglas/frankenphp"
}

group "default" {
    targets = ["bullseye", "alpine"]
}

target "common" {
    platforms = ["linux/amd64", "linux/arm64"]
}

#
# FrankenPHP
#

target "bullseye" {
    inherits = ["common"]
    context = "."
    dockerfile = "Dockerfile"
    tags = ["${REPO_NAME}:bullseye", "${REPO_NAME}:latest"]
}

target "alpine" {
    inherits = ["common"]
    context = "."
    dockerfile = "Dockerfile.alpine"
    tags = ["${REPO_NAME}:alpine"]
}
