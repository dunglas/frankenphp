# FrankenPHP : Serveur d'applications moderne pour PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP est un serveur d'applications moderne pour PHP construit sur la base du serveur web [Caddy](https://caddyserver.com/).

FrankenPHP confère des super pouvoirs à vos applications PHP grâce à ses fonctionnalités étonnantes : [*Early Hints*](https://frankenphp.dev/docs/early-hints/), [mode worker](https://frankenphp.dev/docs/worker/), [capacités en temps réel](https://frankenphp.dev/docs/mercure/), HTTPS automatique, prise en charge de HTTP/2 et HTTP/3...

FrankenPHP fonctionne avec n'importe quelle application PHP et rend vos projets Symfony plus rapides que jamais grâce à l'intégration fournie avec le mode worker.

FrankenPHP peut également être utilisé comme une bibliothèque Go autonome pour intégrer PHP dans n'importe quelle application en utilisant `net/http`.

Vous pouvez découvrir plus en détails ce serveur d’application sur ce replay du Forum PHP 2022 :

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Diapositives" width="600"></a>

## Pour Commencer

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Allez sur `https://localhost`, et profitez !

> [!TIP]
>
> Ne tentez pas d'utiliser `https://127.0.0.1`. Utilisez localhost et acceptez le certificat auto-signé.
> Utilisez [ la variable d'environnement `SERVER_NAME`](docs/config.md#environment-variables) pour changer le domaine à utiliser.

### Binaire autonome

Si vous préférez ne pas utiliser Docker, nous fournissons des binaires autonomes de FrankenPHP pour Linux et macOS
contenant [PHP 8.3](https://www.php.net/releases/8.3/fr.php) et la plupart des extensions PHP populaires : [Télécharger FrankenPHP](https://github.com/dunglas/frankenphp/releases)

Pour servir le contenu du répertoire courant, exécutez :

```console
./frankenphp php-server
```

Vous pouvez également exécuter des scripts en ligne de commande avec :

```console
./frankenphp php-cli /path/to/your/script.php
```

## Documentation

* [Le mode worker](https://frankenphp.dev/docs/worker/)
* [Le support des Early Hints (Code statut 103 HTTP)](https://frankenphp.dev/docs/early-hints/)
* [Temps réel](https://frankenphp.dev/docs/mercure/)
* [Configuration](https://frankenphp.dev/docs/config/)
* [Images Docker](https://frankenphp.dev/docs/docker/)
* [Déploiement en production](docs/production.md)
* [Créer des applications PHP **standalone**, auto-exécutables](https://frankenphp.dev/docs/embed/)
* [Créer un build statique](https://frankenphp.dev/docs/static/)
* [Compiler depuis les sources](https://frankenphp.dev/docs/compile/)
* [Intégration Laravel](https://frankenphp.dev/docs/laravel/)
* [Problèmes connus](https://frankenphp.dev/docs/known-issues/)
* [Application de démo (Symfony) et benchmarks](https://github.com/dunglas/frankenphp-demo)
* [Documentation de la bibliothèque Go](https://pkg.go.dev/github.com/dunglas/frankenphp)
* [Contribuer et débugger](https://frankenphp.dev/docs/contributing/)

## Exemples et squelettes

* [Symfony](https://github.com/dunglas/symfony-docker)
* [API Platform](https://api-platform.com/docs/distribution/)
* [Laravel](https://frankenphp.dev/docs/laravel/)
* [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
* [WordPress](https://github.com/dunglas/frankenphp-wordpress)
* [Drupal](https://github.com/dunglas/frankenphp-drupal)
* [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
* [TYPO3](https://github.com/ochorocho/franken-typo3)
