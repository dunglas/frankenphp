# Kaynak Kodlardan Derleme

Bu doküman, PHP'yi dinamik bir kütüphane olarak yükleyecek bir FrankenPHP yapısının nasıl oluşturulacağını açıklamaktadır.
Önerilen yöntem bu şekildedir.

Alternatif olarak, [statik yapılar oluşturma](static.md) da mümkündür.

## PHP'yi yükleyin

FrankenPHP, PHP 8.2 ve üstü ile uyumludur.

İlk olarak, [PHP'nin kaynaklarını edinin](https://www.php.net/downloads.php) ve bunları çıkarın:

```console
tar xf php-*
cd php-*/
```

Ardından, PHP'yi platformunuz için yapılandırın.

Bu şekilde yapılandırma gereklidir, ancak başka opsiyonlar da ekleyebilirsiniz (örn. ekstra uzantılar)
İhtiyaç halinde.

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

### Mac

Yüklemek için [Homebrew](https://brew.sh/) paket yöneticisini kullanın
`libiconv`, `bison`, `re2c` ve `pkg-config`:

```console
brew install libiconv bison re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Ardından yapılandırma betiğini çalıştırın:

```console
./configure \
    --enable-embed=static \
    --enable-zts \
    --disable-zend-signals \
    --disable-opcache-jit \
    --enable-static \
    --enable-shared=no \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

## PHP Derleyin

Son olarak, PHP'yi derleyin ve kurun:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## İsteğe Bağlı Bağımlılıkları Yükleyin

FrankenPHP'nin bazı özellikleri, yüklenmesi gereken isteğe bağlı bağımlılıklara ihtiyaç duyar.
Bu özellikler, Go derleyicisine derleme etiketleri geçirilerek de devre dışı bırakılabilir.

| Özellik                                                | Bağımlılık                                                           | Devre dışı bırakmak için derleme etiketi |
| ------------------------------------------------------ | -------------------------------------------------------------------- | ---------------------------------------- |
| Brotli sıkıştırma                                      | [Brotli](https://github.com/google/brotli)                           | nobrotli                                 |
| Dosya değişikliğinde işçileri yeniden başlatma         | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher                                |

## Go Uygulamasını Derleyin

Artık Go kütüphanesini kullanabilir ve Caddy yapımızı derleyebilirsiniz:

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main
./install-dependencies.sh
cd caddy/frankenphp
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
go build -tags=nobadger,nomysql,nopgx
```

Bu, Mercure veya Vulcain olmadan bir `frankenphp` ikili dosyası oluşturacaktır. Üretim kullanımı için xcaddy kullanmak daha iyidir.

### Xcaddy kullanımı

Alternatif olarak, FrankenPHP'yi [özel Caddy modülleri](https://caddyserver.com/docs/modules/) ile derlemek için [xcaddy](https://github.com/caddyserver/xcaddy) kullanın:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
    # Ekstra Caddy modüllerini ve FrankenPHP uzantılarını buraya ekleyin
```

> [!TIP]
>
> Eğer musl libc (Alpine Linux'ta varsayılan) ve Symfony kullanıyorsanız,
> varsayılan yığın boyutunu artırmanız gerekebilir.
> Aksi takdirde, şu tarz hatalar alabilirsiniz `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Bunu yapmak için, `XCADDY_GO_BUILD_FLAGS` ortam değişkenini bu şekilde değiştirin
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (yığın boyutunun değerini uygulamanızın ihtiyaçlarına göre değiştirin).
> Daha fazla bilgi için build-static.sh dosyasını kontrol edin.
