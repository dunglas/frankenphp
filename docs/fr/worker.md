# Utilisation des workers FrankenPHP

Démarrez votre application une fois et gardez-la en mémoire.
FrankenPHP traitera les requêtes entrantes en quelques millisecondes.

## Démarrage des scripts workers

### Docker

Définissez la valeur de la variable d'environnement `FRANKENPHP_CONFIG` à `worker /path/to/your/worker/script.php` :

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
frankenphp php-server --worker /path/to/your/worker/script.php
```

Si votre application PHP est [intégrée dans le binaire](embed.md), vous pouvez également ajouter un `Caddyfile` personnalisé dans le répertoire racine de l'application.
Il sera utilisé automatiquement.

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

// En dehors de la boucle pour de meilleures performances (moins de travail effectué)
$handler = static function () use ($myApp) {
    // Appelé lorsqu'une requête est reçue,
    // les superglobales, php://input, etc., sont réinitialisés
    echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
};

for ($nbRequests = 0, $running = true; isset($_SERVER['MAX_REQUESTS']) && ($nbRequests < ((int)$_SERVER['MAX_REQUESTS'])) && $running; ++$nbRequests) {
    $running = \frankenphp_handle_request($handler);

    // Faire quelque chose après l'envoi de la réponse HTTP
    $myApp->terminate();

    // Exécuter le ramasse-miettes pour réduire les chances qu'il soit déclenché au milieu de la génération d'une page
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

Par défaut, 2 workers par CPU sont démarrés.
Vous pouvez également configurer le nombre de workers à démarrer :

```console
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

### Redémarrer le worker après un certain nombre de requêtes

Comme PHP n'a pas été initialement conçu pour des processus de longue durée, de nombreuses bibliothèques et codes anciens présentent encore des fuites de mémoire.
Une solution pour utiliser ce type de code en mode worker est de redémarrer le script worker après avoir traité un certain nombre de requêtes :

Le code du worker précédent permet de configurer un nombre maximal de requêtes à traiter en définissant une variable d'environnement nommée `MAX_REQUESTS`.

## Comportement des superglobales

[Les superglobales PHP](https://www.php.net/manual/fr/language.variables.superglobals.php) (`$_SERVER`, `$_ENV`, `$_GET`...)
se comportent comme suit :

* avant le premier appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs liées au script worker lui-même
* pendant et après l'appel à `frankenphp_handle_request()`, les superglobales contiennent des valeurs générées à partir de la requête HTTP traitée, chaque appel à `frankenphp_handle_request()` change les valeurs des superglobales

Pour accéder aux superglobales du script worker à l'intérieur de la fonction de rappel, vous devez les copier et importer la copie dans le scope de la fonction :

```php
<?php
// Copier la superglobale $_SERVER du worker avant le premier appel à frankenphp_handle_request()
$workerServer = $_SERVER;

$handler = static function () use ($workerServer) {
    var_dump($_SERVER); // $_SERVER lié à la requête
    var_dump($workerServer); // $_SERVER du script worker
};

// ...
```
