# Building Docker Images

Print bake plan:

```
docker buildx bake -f docker-bake.hcl --print
```

Build FrankenPHP images for amd64 locally:

```
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Build FrankenPHP images for arm64 locally:

```
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Build FrankenPHP images from scratch for arm64 & amd64 and push to Docker Hub:

```
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## Resources

* [Bake file definition](https://docs.docker.com/build/customize/bake/file-definition/)
* [docker buildx build](https://docs.docker.com/engine/reference/commandline/buildx_build/)
