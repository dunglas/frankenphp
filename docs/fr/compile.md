# Compiler depuis les sources

Ce document explique comment créer un build FrankenPHP qui chargera PHP comme une bibliothèque dynamique. C'est la méthode recommandée.

En alternative, il est aussi possible de [créer des builds statiques](static.md).

## Installer PHP

FrankenPHP est compatible avec PHP 8.2 et versions ultérieures.

Tout d'abord, [téléchargez les sources de PHP](https://www.php.net/downloads.php) et extrayez-les :

```console
tar xf php-*
cd php-*/
```

Ensuite, configurez PHP pour votre plateforme :

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

Finalement, compilez et installez PHP :

```console
make -j$(nproc)
sudo make install
```

### Mac

Utilisez le gestionnaire de paquets [Homebrew](https://brew.sh/) pour installer `libiconv`, `bison`, `re2c` et `pkg-config` :

```console
brew install libiconv bison re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Puis exécutez le script de configuration :

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

Les options de configuration spécifiées sont nécessaires pour la compilation, mais vous pouvez également inclure d'autres options selon vos besoins, par exemple pour ajouter des extensions supplémentaires.

Finalement, compilez et installez PHP :

```console
make -j$(sysctl -n hw.logicalcpu)
sudo make install
```

## Compiler l'application Go

Vous pouvez maintenant utiliser la bibliothèque Go et compiler notre build Caddy :

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar x
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build
```

### Utiliser xcaddy

Vous pouvez utiliser à la place [xcaddy](https://github.com/caddyserver/xcaddy) pour compiler FrankenPHP avec [des modules Caddy personnalisés](https://caddyserver.com/docs/modules/):

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags '-w -s'" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules here
```

> [!TIP]
>
> Si vous utilisez musl libc (le défaut sur Alpine Linux) et Symfony,
> vous pourriez avoir besoin d'augmenter la taille par défaut de la pile.
> Sinon, vous pourriez rencontrer des erreurs telles que `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> Pour ce faire, modifiez la variable d'environnement `XCADDY_GO_BUILD_FLAGS` en quelque chose comme
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (modifiez la valeur de la taille de la pile selon les besoins de votre application).
