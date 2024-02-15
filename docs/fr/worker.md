# Utilisation des workers FrankenPHP

Démarrez votre application une fois et gardez-la en mémoire.
FrankenPHP gérera les requêtes entrantes en quelques millisecondes.

## Démarrage des scripts de workers

### Docker

Définissez la valeur de la variable d'environnement `FRANKENPHP_CONFIG` sur `worker /path/to/your/worker/script.php` :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker /app/path/to/your/worker/script.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Binaire autonome

Utilisez l'option --worker de la commande php-server pour servir le contenu du répertoire courant en utilisant un worker :

```console
./frankenphp php-server --worker /path/to/your/worker/script.php
```

## Runtime Symfony

Le mode worker de FrankenPHP est pris en charge par le [Composant Runtime de Symfony](https://symfony.com/doc/current/components/runtime.html).
Pour démarrer une application Symfony dans un worker, installez le package FrankenPHP de [PHP Runtime](https://github.com/php-runtime/runtime) :

```console
composer require runtime/frankenphp-symfony
```

Démarrez votre serveur d'application en définissant la variable d'environnement `APP_RUNTIME` pour utiliser le Runtime Symfony de FrankenPHP :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

## Laravel Octane

Voir [la documentation dédiée](laravel.md#laravel-octane).

## Applications Personnalisées

L'exemple suivant montre comment créer votre propre script worker sans dépendre d'une bibliothèque tierce :

```php
<?php
// public/index.php

// Empêcher la terminaison du script worker lorsqu'une connexion client est interrompue
ignore_user_abort(true);

// Démarrer votre application
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

// Gestionnaire en dehors de la boucle pour de meilleures performances (réaliser moins de travail)
$handler = static function () use ($myApp) {
        // Appelé lorsqu'une requête est reçue,
        // les superglobales, php://input, etc., sont réinitialisés
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};
for($nbRequests = 0, $running = true; isset($_SERVER['MAX_REQUESTS']) && ($nbRequests < ((int)$_SERVER['MAX_REQUESTS'])) && $running; ++$nbRequests) {
    $running = \frankenphp_handle_request($handler);

    // Faire quelque chose après l'envoi de la réponse HTTP
    $myApp->terminate();

    // Appeler le garbage collector pour réduire les chances qu'il soit déclenché au milieu de la génération d'une page
    gc_collect_cycles();
}
// Nettoyage
$myApp->shutdown();

```

Ensuite, démarrez votre application et utilisez la variable d'environnement `FRANKENPHP_CONFIG` pour configurer votre worker :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Par défaut, un worker par CPU est démarré.
Vous pouvez également configurer le nombre de workers à démarrer :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Redémarrer le Worker Après un Certain Nombre de Requêtes

Comme PHP n'a pas été initialement conçu pour des processus de longue durée, de nombreuses bibliothèques et codes hérités présentent encore des fuites de mémoire.
Une solution pour utiliser ce type de code en mode worker est de redémarrer le script worker après avoir traité un certain nombre de requêtes :

L'extrait de worker précédent permet de configurer un nombre maximal de requêtes à traiter en définissant une variable d'environnement nommée `MAX_REQUESTS`.

