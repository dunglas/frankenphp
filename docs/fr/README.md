# FrankenPHP : le serveur d'applications PHP moderne, écrit en Go

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP est un serveur d'applications moderne pour PHP construit à partir du serveur web [Caddy](https://caddyserver.com/).

FrankenPHP donne des super-pouvoirs à vos applications PHP grâce à ses fonctionnalités à la pointe : [_Early Hints_](early-hints.md), [mode worker](worker.md), [fonctionnalités en temps réel](mercure.md), HTTPS automatique, prise en charge de HTTP/2 et HTTP/3...

FrankenPHP fonctionne avec n'importe quelle application PHP et rend vos projets Laravel et Symfony plus rapides que jamais grâce à leurs intégrations officielles avec le mode worker.

FrankenPHP peut également être utilisé comme une bibliothèque Go autonome qui permet d'intégrer PHP dans n'importe quelle application en utilisant `net/http`.

Découvrez plus de détails sur ce serveur d’application dans le replay de cette conférence donnée au Forum PHP 2022 :

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Diapositives" width="600"></a>

## Pour Commencer

### Binaire autonome

Si vous préférez ne pas utiliser Docker, nous fournissons des binaires autonomes de FrankenPHP pour Linux et macOS
contenant [PHP 8.4](https://www.php.net/releases/8.4/fr.php) et la plupart des extensions PHP populaires.

Sous Windows, utilisez [WSL](https://learn.microsoft.com/windows/wsl/) pour exécuter FrankenPHP.

[Téléchargez FrankenPHP](https://github.com/php/frankenphp/releases) ou copiez cette ligne dans votre terminal pour installer automatiquement la version appropriée à votre plateforme :

```console
curl https://frankenphp.dev/install.sh | sh
mv frankenphp /usr/local/bin/
```

Pour servir le contenu du répertoire courant, exécutez :

```console
frankenphp php-server
```

Vous pouvez également exécuter des scripts en ligne de commande avec :

```console
frankenphp php-cli /path/to/your/script.php
```

### Docker

Des [images Docker](https://frankenphp.dev/docs/fr/docker/) sont également disponibles :

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Rendez-vous sur `https://localhost`, c'est parti !

> [!TIP]
>
> Ne tentez pas d'utiliser `https://127.0.0.1`. Utilisez `https://localhost` et acceptez le certificat auto-signé.
> Utilisez [la variable d'environnement `SERVER_NAME`](config.md#variables-denvironnement) pour changer le domaine à utiliser.

### Homebrew

FrankenPHP est également disponible sous forme de paquet [Homebrew](https://brew.sh) pour macOS et Linux.

Pour l'installer :

```console
brew install dunglas/frankenphp/frankenphp
```

Pour servir le contenu du répertoire courant, exécutez :

```console
frankenphp php-server
```

## Documentation

- [Le mode classique](classic.md)
- [Le mode worker](worker.md)
- [Le support des Early Hints (code de statut HTTP 103)](early-hints.md)
- [Temps réel](mercure.md)
- [Servir efficacement les fichiers statiques volumineux](x-sendfile.md)
- [Configuration](config.md)
- [Écrire des extensions PHP en Go](extensions.md)
- [Images Docker](docker.md)
- [Déploiement en production](production.md)
- [Optimisation des performances](performance.md)
- [Créer des applications PHP **standalone**, auto-exécutables](embed.md)
- [Créer un build statique](static.md)
- [Compiler depuis les sources](compile.md)
- [Surveillance de FrankenPHP](metrics.md)
- [Intégration Laravel](laravel.md)
- [Problèmes connus](known-issues.md)
- [Application de démo (Symfony) et benchmarks](https://github.com/dunglas/frankenphp-demo)
- [Documentation de la bibliothèque Go](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Contribuer et débugger](CONTRIBUTING.md)

## Exemples et squelettes

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/distribution/)
- [Laravel](laravel.md)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
