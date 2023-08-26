# Create a Static Build

Instead of using a local installation of the PHP library,
it's possible to create a static build of FrankenPHP thanks to the great [static-php-cli project](https://github.com/crazywhalecc/static-php-cli) (despite its name, this project support all SAPIs, not only CLI).

With this method, a single, portable, binary will contain the PHP interpreter, the Caddy web server and FrankenPHP!

## Build Instructions

```console
git clone https://github.com/dunglas/static-php-cli.git --branch=feat/embed
cd static-php-cli
bin/spc-alpine-docker fetch --with-php=8.2 -A
bin/spc-alpine-docker build --build-embed --enable-zts --debug "bcmath,calendar,ctype,curl,dba,dom,exif,filter,fileinfo,gd,iconv,mbstring,mbregex,mysqli,mysqlnd,openssl,pcntl,pdo,pdo_mysql,pdo_sqlite,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,tokenizer,xml,xmlreader,xmlwriter,zip,zlib,apcu"
export CGO_CFLAGS="$(sh $PWD/source/php-src/scripts/php-config --includes | sed s#-I/include/php#-I$PWD/source/php-src#g)"
export CGO_LDFLAGS="-L$PWD/source/php-src/libs $(sh $PWD/source/php-src/scripts/php-config --ldflags) $(sh $PWD/source/php-src/scripts/php-config --libs | sed 's/-lgcc_s//g')"

cd ..

git clone https://github.com/dunglas/frankenphp
cd frankenphp/caddy/frankenphp
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie"
```

The `frankenphp` file in the current directory is a static build of FrankenPHP!

Customize the extensions to build using [the command generator](https://static-php-cli.zhamao.me/en/guide/cli-generator.html).
