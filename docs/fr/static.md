# Créer un Build Statique

Au lieu d'utiliser une installation locale de la bibliothèque PHP, il est possible de créer un build statique de FrankenPHP grâce au formidable projet [static-php-cli](https://github.com/crazywhalecc/static-php-cli) (malgré son nom, ce projet prend en charge tous les SAPIs, pas seulement CLI).

Avec cette méthode, un binaire portable unique contiendra l'interpréteur PHP, le serveur web Caddy et FrankenPHP !

FrankenPHP prend également en charge [l'incorporation de l'application PHP dans le binaire statique](embed.md).

## Linux

Nous fournissons une image Docker pour créer un binaire statique Linux :

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

Le binaire statique résultant est nommé frankenphp et est disponible dans le répertoire courant.

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

Voir aussi : [personnaliser la construction](#customizing-the-build)

### Jeton GitHub

Si vous atteignez la limite de taux d'appels de l'API GitHub, définissez un jeton d'accès personnel GitHub dans une variable d'environnement nommée `GITHUB_TOKEN` :

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

Exécutez le script suivant pour créer un binaire statique pour macOS (vous devez avoir [Homebrew](https://brew.sh/) installé) :

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

Note : ce script fonctionne également sur Linux (et probablement sur d'autres Unix) et est utilisé en interne par le constructeur statique basé sur Docker que nous fournissons.

## Personnalisation de la construction

Les variables d'environnement suivantes peuvent être transmises à `docker build` et au script `build-static.sh` pour personnaliser la construction statique :

* `FRANKENPHP_VERSION` : la version de FrankenPHP à utiliser
* `PHP_VERSION` : la version de PHP à utiliser
* `PHP_EXTENSIONS` : les extensions PHP à construire ([liste des extensions prises en charge](https://static-php.dev/en/guide/extensions.html))
* `PHP_EXTENSION_LIBS` : bibliothèques supplémentaires à construire qui ajoutent des fonctionnalités supplémentaires aux extensions
* `EMBED` : chemin de l'application PHP à intégrer dans le binaire
* `CLEAN` : lorsqu'il est défini, libphp et toutes ses dépendances sont construites à partir de zéro (pas de cache)
* `DEBUG_SYMBOLS` : lorsqu'il est défini, les symboles de débogage ne seront pas supprimés et seront ajoutés dans le binaire
* `RELEASE` : (uniquement pour les mainteneurs) lorsqu'elle est définie, le binaire résultant sera téléchargé sur GitHub