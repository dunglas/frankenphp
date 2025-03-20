#!/bin/bash

set -o errexit
set -x

if ! type "git" >/dev/null 2>&1; then
	echo "The \"git\" command must be installed."
	exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"

# Supported variables:
# - PHP_VERSION: PHP version to build (default: "8.4")
# - PHP_EXTENSIONS: PHP extensions to build (default: ${defaultExtensions} set below)
# - PHP_EXTENSION_LIBS: PHP extension libraries to build (default: ${defaultExtensionLibs} set below)
# - FRANKENPHP_VERSION: FrankenPHP version (default: current Git commit)
# - EMBED: Path to the PHP app to embed (default: none)
# - DEBUG_SYMBOLS: Enable debug symbols if set to 1 (default: none)
# - MIMALLOC: Use mimalloc as the allocator if set to 1 (default: none)
# - XCADDY_ARGS: Additional arguments to pass to xcaddy
# - RELEASE: [maintainer only] Create a GitHub release if set to 1 (default: none)

# - SPC_REL_TYPE: Release type to download (accept "source" and "binary", default: "source")
# - SPC_OPT_BUILD_ARGS: Additional arguments to pass to spc build
# - SPC_OPT_DOWNLOAD_ARGS: Additional arguments to pass to spc download
# - SPC_LIBC: Set to glibc to build with GNU toolchain (default: musl)

# init spc command, if we use spc binary, just use it instead of fetching source
if [ -z "${SPC_REL_TYPE}" ]; then
	SPC_REL_TYPE="source"
fi
# init spc build additional args
if [ -z "${SPC_OPT_BUILD_ARGS}" ]; then
	SPC_OPT_BUILD_ARGS="--debug"
fi
# init spc download additional args
if [ -z "${SPC_OPT_DOWNLOAD_ARGS}" ]; then
	if [ "${os}" = "linux" ] && [ "${SPC_LIBC}" = "glibc" ]; then
		SPC_OPT_DOWNLOAD_ARGS="--debug --ignore-cache-sources=php-src"
	else
		SPC_OPT_DOWNLOAD_ARGS="--prefer-pre-built --debug --ignore-cache-sources=php-src"
	fi
fi
# if we need debug symbols, disable strip
if [ -n "${DEBUG_SYMBOLS}" ]; then
		SPC_OPT_BUILD_ARGS="${SPC_OPT_BUILD_ARGS} --no-strip"
fi
# php version to build
if [ -z "${PHP_VERSION}" ]; then
	export PHP_VERSION="8.4"
fi
# default extension set
defaultExtensions="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,ftp,gd,gmp,gettext,iconv,igbinary,imagick,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,parallel,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,protobuf,readline,redis,session,shmop,simplexml,soap,sockets,sodium,sqlite3,ssh2,sysvmsg,sysvsem,sysvshm,tidy,tokenizer,xlswriter,xml,xmlreader,xmlwriter,zip,zlib,yaml,zstd"
if [ "${os}" != "linux" ] || [ "${SPC_LIBC}" = "glibc" ]; then
	defaultExtensions="${defaultExtensions},ffi"
fi
defaultExtensionLibs="bzip2,freetype,libavif,libjpeg,liblz4,libwebp,libzip,nghttp2"

md5binary="md5sum"
if [ "${os}" = "darwin" ]; then
	os="mac"
	md5binary="md5 -q"
fi

if [ "${os}" = "linux" ] && ! type "cmake" >/dev/null 2>&1; then
	echo "The \"cmake\" command must be installed."
	exit 1
fi

if [ "${os}" = "linux" ] && { [[ "${arch}" =~ "aarch" ]] || [[ "${arch}" =~ "arm" ]]; }; then
	fpic="-fPIC"
	fpie="-fPIE"

	if [ -z "${DEBUG_SYMBOLS}" ]; then
		export SPC_PHP_DEFAULT_OPTIMIZE_CFLAGS="-g -fstack-protector-strong -fPIC -fPIE -Os -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	fi
else
	fpic="-fpic"
	fpie="-fpie"
fi

if [ -z "${FRANKENPHP_VERSION}" ]; then
	FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
	export FRANKENPHP_VERSION
elif [ -d ".git/" ]; then
	CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
	export CURRENT_REF

	if echo "${FRANKENPHP_VERSION}" | grep -F -q "."; then
		# Tag
	
		# Trim "v" prefix if any
		FRANKENPHP_VERSION=${FRANKENPHP_VERSION#v}
		export FRANKENPHP_VERSION
	
		git checkout "v${FRANKENPHP_VERSION}"
	else
		git checkout "${FRANKENPHP_VERSION}"
	fi
fi

bin="frankenphp-${os}-${arch}"

if [ -n "${CLEAN}" ]; then
	rm -Rf dist/
	go clean -cache
fi

cache_key="${PHP_VERSION}-${PHP_EXTENSIONS}-${PHP_EXTENSION_LIBS}"

# Build libphp if necessary
if [ -f dist/cache_key ] && [ "$(cat dist/cache_key)" = "${cache_key}" ] && [ -f "dist/static-php-cli/buildroot/lib/libphp.a" ]; then
	cd dist/static-php-cli
else
	mkdir -p dist/
	cd dist/
	echo -n "${cache_key}" >cache_key

	if type "brew" >/dev/null 2>&1; then
		if ! type "composer" >/dev/null; then
			packages="composer"
		fi
		if ! type "go" >/dev/null 2>&1; then
			packages="${packages} go"
		fi
		if [ -n "${RELEASE}" ] && ! type "gh" >/dev/null 2>&1; then
			packages="${packages} gh"
		fi

		if [ -n "${packages}" ]; then
			# shellcheck disable=SC2086
			brew install --formula --quiet ${packages}
		fi
	fi

	if [ "${SPC_REL_TYPE}" = "binary" ]; then
		mkdir static-php-cli/
		cd static-php-cli/
		curl -o spc -fsSL https://dl.static-php.dev/static-php-cli/spc-bin/nightly/spc-linux-$(uname -m)
		chmod +x spc
		spcCommand="./spc"
	elif [ -d "static-php-cli/src" ]; then
		cd static-php-cli/
		git pull
		composer install --no-dev -a
		spcCommand="./bin/spc"
	else
		git clone --depth 1 https://github.com/crazywhalecc/static-php-cli --branch main
		cd static-php-cli/
		composer install --no-dev -a
		spcCommand="./bin/spc"
	fi

	# extensions to build
	if [ -z "${PHP_EXTENSIONS}" ]; then
		# enable EMBED mode, first check if project has dumped extensions
		if [ -n "${EMBED}" ] && [ -f "${EMBED}/composer.json" ] && [ -f "${EMBED}/composer.lock" ] && [ -f "${EMBED}/vendor/installed.json" ]; then
			cd "${EMBED}"
			# read the extensions using spc dump-extensions
			PHP_EXTENSIONS=$(${spcCommand} dump-extensions "${EMBED}" --format=text --no-dev --no-ext-output="${defaultExtensions}")
		else
			PHP_EXTENSIONS="${defaultExtensions}"
		fi
	fi
	# additional libs to build
	if [ -z "${PHP_EXTENSION_LIBS}" ]; then
		PHP_EXTENSION_LIBS="${defaultExtensionLibs}"
	fi
	# The Brotli library must always be built as it is required by http://github.com/dunglas/caddy-cbrotli
	if ! echo "${PHP_EXTENSION_LIBS}" | grep -q "\bbrotli\b"; then
		PHP_EXTENSION_LIBS="${PHP_EXTENSION_LIBS},brotli"
	fi

	${spcCommand} doctor --auto-fix
	${spcCommand} download --with-php="${PHP_VERSION}" --for-extensions="${PHP_EXTENSIONS}" --for-libs="${PHP_EXTENSION_LIBS}" ${SPC_OPT_DOWNLOAD_ARGS}
	# shellcheck disable=SC2086
	${spcCommand} build --enable-zts --build-embed ${SPC_OPT_BUILD_ARGS} "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}"
fi

if ! type "go" >/dev/null 2>&1; then
	echo "The \"go\" command must be installed."
	exit 1
fi

XCADDY_COMMAND="xcaddy"
if ! type "$XCADDY_COMMAND" >/dev/null 2>&1; then
	go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
	XCADDY_COMMAND="$(go env GOPATH)/bin/xcaddy"
fi

curlGitHubHeaders=(--header "X-GitHub-Api-Version: 2022-11-28")
if [ "${GITHUB_TOKEN}" ]; then
	curlGitHubHeaders+=(--header "Authorization: Bearer ${GITHUB_TOKEN}")
fi

# Compile e-dant/watcher as a static library
mkdir -p watcher
cd watcher
curl -f --retry 5 "${curlGitHubHeaders[@]}" https://api.github.com/repos/e-dant/watcher/releases/latest |
grep tarball_url |
awk '{ print $2 }' |
sed 's/,$//' |
sed 's/"//g' |
xargs curl -fL --retry 5 "${curlGitHubHeaders[@]}" |
tar xz --strip-components 1
cd watcher-c
if [ -z "${CC}" ]; then
		watcherCC=cc
else
		watcherCC="${CC}"
fi
${watcherCC} -c -o libwatcher-c.o ./src/watcher-c.cpp -I ./include -I ../include -std=c++17 -Wall -Wextra "${fpic}"
ar rcs libwatcher-c.a libwatcher-c.o
cp libwatcher-c.a ../../buildroot/lib/libwatcher-c.a
mkdir -p ../../buildroot/include/wtr
cp -R include/wtr/watcher-c.h ../../buildroot/include/wtr/watcher-c.h
cd ../../

# See https://github.com/docker-library/php/blob/master/8.3/alpine3.20/zts/Dockerfile#L53-L55
CGO_CFLAGS="-DFRANKENPHP_VERSION=${FRANKENPHP_VERSION} -I${PWD}/buildroot/include/ $(${spcCommand} spc-config "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}" --includes)"
if [ -n "${DEBUG_SYMBOLS}" ]; then
	CGO_CFLAGS="-g ${CGO_CFLAGS}"
else
	CGO_CFLAGS="-fstack-protector-strong ${fpic} ${fpie} -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64 ${CGO_CFLAGS}"
fi
export CGO_CFLAGS
export CGO_CPPFLAGS="${CGO_CFLAGS}"

if [ "${os}" = "mac" ]; then
	export CGO_LDFLAGS="-framework CoreFoundation -framework SystemConfiguration"
elif [ "${os}" = "linux" ] && [ -z "${DEBUG_SYMBOLS}" ]; then
	CGO_LDFLAGS="-Wl,-O1 -pie"
fi
if [ "${os}" = "linux" ] && [ "${SPC_LIBC}" = "glibc" ]; then
	CGO_LDFLAGS="${CGO_LDFLAGS} -Wl,--allow-multiple-definition -Wl,--export-dynamic"
fi

CGO_LDFLAGS="${CGO_LDFLAGS} ${PWD}/buildroot/lib/libbrotlicommon.a ${PWD}/buildroot/lib/libbrotlienc.a ${PWD}/buildroot/lib/libbrotlidec.a ${PWD}/buildroot/lib/libwatcher-c.a $(${spcCommand} spc-config "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}" --libs)"
if [ "${os}" = "linux" ]; then
	if echo "${PHP_EXTENSIONS}" | grep -qE "\b(intl|imagick|grpc|v8js|protobuf|mongodb|tbb)\b"; then
		CGO_LDFLAGS="${CGO_LDFLAGS} -lstdc++"
	fi
	if [ "${SPC_LIBC}" = "glibc" ]; then
		CGO_LDFLAGS=$(echo "$CGO_LDFLAGS" | sed 's|-lphp|-Wl,--whole-archive -lphp -Wl,--no-whole-archive|g')
		ar d ${PWD}/buildroot/lib/libphp.a $(ar t ${PWD}/buildroot/lib/libphp.a | grep '\.a$')
	fi
fi

export CGO_LDFLAGS

LIBPHP_VERSION="$(./buildroot/bin/php-config --version)"

cd ../

if [ "${os}" = "linux" ]; then
	if [ -n "${MIMALLOC}" ]; then
		# Replace musl's mallocng by mimalloc
		# The default musl allocator is slow, especially when used by multi-threaded apps,
		# and triggers weird bugs
		# Adapted from https://www.tweag.io/blog/2023-08-10-rust-static-link-with-mimalloc/

		echo 'The USE_MIMALLOC environment variable is EXPERIMENTAL.'
		echo 'This option can be removed or its behavior modified at any time.'

		if [ ! -f "mimalloc/out/libmimalloc.a" ]; then
			if [ -d "mimalloc" ]; then
				cd mimalloc/
				git reset --hard
				git clean -xdf
				git fetch --tags
			else
				git clone https://github.com/microsoft/mimalloc.git
				cd mimalloc/
			fi

			# mimalloc version must be compatible with version used in tweag/rust-alpine-mimalloc
			git checkout v3.0.1

			curl -fL --retry 5 https://raw.githubusercontent.com/tweag/rust-alpine-mimalloc/1a756444a5c1484d26af9cd39187752728416ba8/mimalloc.diff -o mimalloc.diff
			patch -p1 <mimalloc.diff

			mkdir -p out/
			cd out/
			if [ -n "${DEBUG_SYMBOLS}" ]; then
				cmake \
					-DCMAKE_BUILD_TYPE=Debug \
					-DMI_BUILD_SHARED=OFF \
					-DMI_BUILD_OBJECT=OFF \
					-DMI_BUILD_TESTS=OFF \
					../
			else
				cmake \
					-DCMAKE_BUILD_TYPE=Release \
					-DMI_BUILD_SHARED=OFF \
					-DMI_BUILD_OBJECT=OFF \
					-DMI_BUILD_TESTS=OFF \
					../
			fi
			make -j"$(nproc || true)"

			cd ../../
		fi

		if [ -n "${DEBUG_SYMBOLS}" ]; then
			libmimalloc_path=mimalloc/out/libmimalloc-debug.a
		else
			libmimalloc_path=mimalloc/out/libmimalloc.a
		fi

		# Patch musl library to use mimalloc
		for libc_path in "/usr/local/musl/lib/libc.a" "/usr/local/musl/$(uname -m)-linux-musl/lib/libc.a" "/usr/lib/libc.a"; do
			if [ ! -f "${libc_path}" ] || [ -f "${libc_path}.unpatched" ]; then
				continue
			fi

			{
				echo "CREATE libc.a"
				echo "ADDLIB ${libc_path}"
				echo "DELETE aligned_alloc.lo calloc.lo donate.lo free.lo libc_calloc.lo lite_malloc.lo malloc.lo malloc_usable_size.lo memalign.lo posix_memalign.lo realloc.lo reallocarray.lo valloc.lo"
				echo "ADDLIB ${libmimalloc_path}"
				echo "SAVE"
			} | ar -M
			mv "${libc_path}" "${libc_path}.unpatched"
			mv libc.a "${libc_path}"
		done
	fi

	# Increase the default stack size to prevents issues with code including many files such as Symfony containers
	extraExtldflags="-Wl,-z,stack-size=0x80000"
fi

if [ -z "${DEBUG_SYMBOLS}" ]; then
	extraLdflags="-w -s"
fi

cd ../

# Embed PHP app, if any
if [ -n "${EMBED}" ] && [ -d "${EMBED}" ]; then
	tar -cf app.tar -C "${EMBED}" .
	${md5binary} app.tar | awk '{printf $1}' >app_checksum.txt
fi

if [ -z "${XCADDY_ARGS}" ]; then
	XCADDY_ARGS="--with github.com/dunglas/caddy-cbrotli --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy"
fi

XCADDY_DEBUG=0
if [ -n "${DEBUG_SYMBOLS}" ]; then
	XCADDY_DEBUG=1
fi

go env
cd caddy/
if [ -z "${SPC_LIBC}" ] || [ "${SPC_LIBC}" = "musl" ]; then
	xcaddyGoBuildFlags="-buildmode=pie -tags cgo,netgo,osusergo,static_build,nobadger,nomysql,nopgx -ldflags \"-linkmode=external -extldflags '-static-pie ${extraExtldflags}' ${extraLdflags} -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP ${FRANKENPHP_VERSION} PHP ${LIBPHP_VERSION} Caddy'\""
elif [ "${SPC_LIBC}" = "glibc" ]; then
	xcaddyGoBuildFlags="-buildmode=pie -tags cgo,netgo,osusergo,nobadger,nomysql,nopgx -ldflags \"-linkmode=external -extldflags '-pie ${extraExtldflags}' ${extraLdflags} -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP ${FRANKENPHP_VERSION} PHP ${LIBPHP_VERSION} Caddy'\""
fi

# shellcheck disable=SC2086
CGO_ENABLED=1 \
	XCADDY_GO_BUILD_FLAGS=${xcaddyGoBuildFlags} \
	XCADDY_DEBUG="${XCADDY_DEBUG}" \
	${XCADDY_COMMAND} build \
	--output "../dist/${bin}" \
	${XCADDY_ARGS} \
	--with github.com/dunglas/frankenphp=.. \
	--with github.com/dunglas/frankenphp/caddy=.
cd ..

if [ -d "${EMBED}" ]; then
	truncate -s 0 app.tar
	truncate -s 0 app_checksum.txt
fi

if type "upx" >/dev/null 2>&1 && [ -z "${DEBUG_SYMBOLS}" ] && [ -z "${NO_COMPRESS}" ]; then
	upx --best "dist/${bin}"
fi

"dist/${bin}" version
"dist/${bin}" build-info

if [ -n "${RELEASE}" ]; then
	gh release upload "v${FRANKENPHP_VERSION}" "dist/${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
	git checkout "${CURRENT_REF}"
fi
