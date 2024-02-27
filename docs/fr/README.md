# FrankenPHP : le serveur d'applications PHP moderne, écrit en Go

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP est un serveur d'applications moderne pour PHP construit à partir du serveur web [Caddy](https://caddyserver.com/).

FrankenPHP donne des super-pouvoirs à vos applications PHP grâce à ses fonctionnalités à la pointe : [*Early Hints*](https://frankenphp.dev/docs/fr/early-hints/), [mode worker](https://frankenphp.dev/docs/fr/worker/), [fonctionnalités en temps réel](https://frankenphp.dev/docs/fr/mercure/), HTTPS automatique, prise en charge de HTTP/2 et HTTP/3...

FrankenPHP fonctionne avec n'importe quelle application PHP et rend vos projets Laravel et Symfony plus rapides que jamais grâce à leurs intégrations officielles avec le mode worker.

FrankenPHP peut également être utilisé comme une bibliothèque Go autonome qui permet d'intégrer PHP dans n'importe quelle application en utilisant `net/http`.

Découvrez plus de détails sur ce serveur d’application dans le replay de cette conférence donnée au Forum PHP 2022 :

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Diapositives" width="600"></a>

## Pour Commencer

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Rendez-vous sur `https://localhost`, c'est parti !

> [!TIP]
>
> Ne tentez pas d'utiliser `https://127.0.0.1`. Utilisez localhost et acceptez le certificat auto-signé.
> Utilisez [la variable d'environnement `SERVER_NAME`](docs/config.md#environment-variables) pour changer le domaine à utiliser.

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

* [Le mode worker](https://frankenphp.dev/docs/fr/worker/)
* [Le support des Early Hints (code de statut HTTP 103)](https://frankenphp.dev/docs/fr/early-hints/)
* [Temps réel](https://frankenphp.dev/docs/mercure/)
* [Configuration](https://frankenphp.dev/docs/config/)
* [Images Docker](https://frankenphp.dev/docs/docker/)
* [Déploiement en production](docs/production.md)
* [Créer des applications PHP **standalone**, auto-exécutables](https://frankenphp.dev/docs/fr/embed/)
* [Créer un build statique](https://frankenphp.dev/docs/fr/static/)
* [Compiler depuis les sources](https://frankenphp.dev/docs/fr/compile/)
* [Intégration Laravel](https://frankenphp.dev/docs/fr/laravel/)
* [Problèmes connus](https://frankenphp.dev/docs/fr/known-issues/)
* [Application de démo (Symfony) et benchmarks](https://github.com/dunglas/frankenphp-demo)
* [Documentation de la bibliothèque Go](https://pkg.go.dev/github.com/dunglas/frankenphp)
* [Contribuer et débugger](https://frankenphp.dev/docs/fr/contributing/)

## Exemples et squelettes

* [Symfony](https://github.com/dunglas/symfony-docker)
* [API Platform](https://api-platform.com/docs/distribution/)
* [Laravel](https://frankenphp.dev/docs/laravel/)
* [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
* [WordPress](https://github.com/dunglas/frankenphp-wordpress)
* [Drupal](https://github.com/dunglas/frankenphp-drupal)
* [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
* [TYPO3](https://github.com/ochorocho/franken-typo3)
