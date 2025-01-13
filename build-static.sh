#!/bin/bash

set -o errexit
set -x

if ! type "git" >/dev/null 2>&1; then
	echo "The \"git\" command must be installed."
	exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
# FIXME: re-enable PHP errors when SPC will be compatible with PHP 8.4
spcCommand="php -ddisplay_errors=Off ./bin/spc"
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

if [ -z "${PHP_EXTENSIONS}" ]; then
	if [ -n "${EMBED}" ] && [ -f "${EMBED}/composer.json" ]; then
		cd "${EMBED}"
		# read the composer.json file and extract the required PHP extensions
		# remove internal extensions from the list: https://github.com/crazywhalecc/static-php-cli/blob/4b16631d45a57370b4747df15c8f105130e96d03/src/globals/defines.php#L26-L34
		PHP_EXTENSIONS="$(composer check-platform-reqs --no-dev 2>/dev/null | grep ^ext | sed -e 's/^ext-core//' -e 's/^ext-hash//' -e 's/^ext-json//' -e 's/^ext-pcre//' -e 's/^ext-reflection//' -e 's/^ext-spl//' -e 's/^ext-standard//' -e 's/^ext-//' -e 's/ .*//' | xargs | tr ' ' ',')"
		export PHP_EXTENSIONS
		cd -
	else
		export PHP_EXTENSIONS="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,ftp,gd,gmp,gettext,iconv,igbinary,imagick,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,parallel,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,protobuf,readline,redis,session,shmop,simplexml,soap,sockets,sodium,sqlite3,ssh2,sysvmsg,sysvsem,sysvshm,tidy,tokenizer,xlswriter,xml,xmlreader,xmlwriter,zip,zlib,yaml,zstd"
	fi
fi

if [ -z "${PHP_EXTENSION_LIBS}" ]; then
	export PHP_EXTENSION_LIBS="bzip2,freetype,libavif,libjpeg,liblz4,libwebp,libzip,nghttp2"
fi

# The Brotli library must always be built as it is required by http://github.com/dunglas/caddy-cbrotli
if ! echo "${PHP_EXTENSION_LIBS}" | grep -q "\bbrotli\b"; then
	export PHP_EXTENSION_LIBS="${PHP_EXTENSION_LIBS},brotli"
fi

if [ -z "${PHP_VERSION}" ]; then
	export PHP_VERSION="8.4"
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

	if [ -d "static-php-cli/" ]; then
		cd static-php-cli/
		git pull
	else
		git clone --depth 1 https://github.com/crazywhalecc/static-php-cli
		cd static-php-cli/
	fi

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

	composer install --no-dev -a

	if [ "${os}" = "linux" ]; then
		extraOpts="--disable-opcache-jit"
	fi

	if [ -n "${DEBUG_SYMBOLS}" ]; then
		extraOpts="${extraOpts} --no-strip"
	fi

	${spcCommand} doctor --auto-fix
	${spcCommand} download --with-php="${PHP_VERSION}" --for-extensions="${PHP_EXTENSIONS}" --for-libs="${PHP_EXTENSION_LIBS}" --ignore-cache-sources=php-src --prefer-pre-built
	# shellcheck disable=SC2086
	${spcCommand} build --debug --enable-zts --build-embed ${extraOpts} "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}"
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
cc -c -o libwatcher-c.o ./src/watcher-c.cpp -I ./include -I ../include -std=c++17 -Wall -Wextra "${fpic}"
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

CGO_LDFLAGS="${CGO_LDFLAGS} ${PWD}/buildroot/lib/libbrotlicommon.a ${PWD}/buildroot/lib/libbrotlienc.a ${PWD}/buildroot/lib/libbrotlidec.a ${PWD}/buildroot/lib/libwatcher-c.a $(${spcCommand} spc-config "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}" --libs)"
if [ "${os}" = "linux" ]; then
	if echo "${PHP_EXTENSIONS}" | grep -qE "\b(intl|imagick|grpc|v8js|protobuf|mongodb|tbb)\b"; then
		CGO_LDFLAGS="${CGO_LDFLAGS} -lstdc++"
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

			git checkout "$(git describe --tags "$(git rev-list --tags --max-count=1 || true)" || true)"

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
# shellcheck disable=SC2086
CGO_ENABLED=1 \
	XCADDY_GO_BUILD_FLAGS="-buildmode=pie -tags cgo,netgo,osusergo,static_build,nobadger,nomysql,nopgx -ldflags \"-linkmode=external -extldflags '-static-pie ${extraExtldflags}' ${extraLdflags} -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP ${FRANKENPHP_VERSION} PHP ${LIBPHP_VERSION} Caddy'\"" \
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

if [ -n "${RELEASE}" ]; then
	gh release upload "v${FRANKENPHP_VERSION}" "dist/${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
	git checkout "${CURRENT_REF}"
fi
