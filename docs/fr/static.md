# Créer un binaire statique

Au lieu d'utiliser une installation locale de la bibliothèque PHP, il est possible de créer un build statique de FrankenPHP grâce à l'excellent projet [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (malgré son nom, ce projet prend en charge tous les SAPIs, pas seulement CLI).

Avec cette méthode, un binaire portable unique contiendra l'interpréteur PHP, le serveur web Caddy et FrankenPHP !

FrankenPHP permet également [d'embarquer l'application PHP dans le binaire statique](embed.md).

## Linux

Nous fournissons une image Docker pour créer un binaire statique pour Linux :

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

Le binaire statique résultant est nommé `frankenphp`, et il est disponible dans le répertoire courant.

Si vous souhaitez construire le binaire statique sans Docker, regardez les instructions pour macOS, qui fonctionnent également pour Linux.

### Extensions personnalisées

Par défaut, la plupart des extensions PHP populaires sont compilées.

Pour réduire la taille du binaire et diminuer la surface d'attaque, vous pouvez choisir la liste des extensions à construire en utilisant l'argument Docker `PHP_EXTENSIONS`.

Par exemple, exécutez la commande suivante pour ne construire que l'extension `opcache` :

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

Pour ajouter des bibliothèques permettant des fonctionnalités supplémentaires aux extensions que vous avez activées, vous pouvez utiliser l'argument Docker `PHP_EXTENSION_LIBS` :

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

### Modules supplémentaires de Caddy

Pour ajouter des modules Caddy supplémentaires ou passer d'autres arguments à [xcaddy](https://github.com/caddyserver/xcaddy), utilisez l'argument Docker `XCADDY_ARGS` :

```console
docker buildx bake \
  --load \
  --set static-builder.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder
```

Dans cet exemple, nous ajoutons le module de cache HTTP [Souin](https://souin.io) pour Caddy ainsi que les modules [Mercure](https://mercure.rocks) et [Vulcain](https://vulcain.rocks).

> [!TIP]
>
> Les modules Mercure et Vulcain sont inclus par défaut si `XCADDY_ARGS` est vide ou n'est pas défini.
> Si vous personnalisez la valeur de `XCADDY_ARGS`, vous devez les inclure explicitement si vous voulez qu'ils soient inclus.

Voir aussi comment [personnaliser la construction](#personnalisation-de-la-construction)

### Jeton GitHub

Si vous atteignez la limite de taux d'appels de l'API GitHub, définissez un jeton d'accès personnel GitHub dans une variable d'environnement nommée `GITHUB_TOKEN` :

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

Exécutez le script suivant pour créer un binaire statique pour macOS (vous devez avoir [Homebrew](https://brew.sh/) d'installé) :

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

Note : ce script fonctionne également sur Linux (et probablement sur d'autres Unix) et est utilisé en interne par le builder statique basé sur Docker que nous fournissons.

## Personnalisation de la construction

Les variables d'environnement suivantes peuvent être transmises à `docker build` et au script `build-static.sh` pour personnaliser la construction statique :

* `FRANKENPHP_VERSION` : la version de FrankenPHP à utiliser
* `PHP_VERSION` : la version de PHP à utiliser
* `PHP_EXTENSIONS` : les extensions PHP à construire ([liste des extensions prises en charge](https://static-php.dev/en/guide/extensions.html))
* `PHP_EXTENSION_LIBS` : bibliothèques supplémentaires à construire qui ajoutent des fonctionnalités aux extensions
* `XCADDY_ARGS` : arguments à passer à [xcaddy](https://github.com/caddyserver/xcaddy), par exemple pour ajouter des modules Caddy supplémentaires
* `EMBED` : chemin de l'application PHP à intégrer dans le binaire
* `CLEAN` : lorsque défini, `libphp` et toutes ses dépendances sont construites à partir de zéro (pas de cache)
* `DEBUG_SYMBOLS` : lorsque défini, les symboles de débogage ne seront pas supprimés et seront ajoutés dans le binaire
* `NO_COMPRESS`: ne pas compresser le binaire avec UPX
* `MIMALLOC`: (expérimental, Linux seulement) remplace l'allocateur mallocng de musl par [mimalloc](https://github.com/microsoft/mimalloc) pour des performances améliorées
* `RELEASE` : (uniquement pour les mainteneurs) lorsque défini, le binaire résultant sera uploadé sur GitHub
