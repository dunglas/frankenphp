# Laravel

## Docker

Déployer une application web [Laravel](https://laravel.com) avec FrankenPHP est très facile.
Il suffit de monter le projet dans le répertoire `/app` de l'image Docker officielle.

Exécutez cette commande depuis le répertoire principal de votre application Laravel :

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

Et profitez !

## Installation Locale

Vous pouvez également exécuter vos projets Laravel avec FrankenPHP depuis votre machine locale :

1. [Téléchargez le binaire correspondant à votre système](README.md#binaire-autonome)
2. Ajoutez la configuration suivante dans un fichier nommé `Caddyfile` placé dans le répertoire racine de votre projet Laravel :

 	```caddyfile
    {
     frankenphp
    }

    # Le nom de domaine de votre serveur
    localhost {
     # Définir le répertoire racine sur le dossier public/
     root * public/
     # Autoriser la compression (optionnel)
     encode zstd br gzip
     # Exécuter les scripts PHP du dossier public/ et servir les assets
     php_server
    }
    ```

3. Démarrez FrankenPHP depuis le répertoire racine de votre projet Laravel : `frankenphp run`

## Laravel Octane

Octane peut être installé via le gestionnaire de paquets Composer :

```console
composer require laravel/octane
```

Après avoir installé Octane, vous pouvez exécuter la commande Artisan `octane:install`, qui installera le fichier de configuration d'Octane dans votre application :

```console
php artisan octane:install --server=frankenphp
```

Le serveur Octane peut être démarré via la commande Artisan `octane:frankenphp`.

```console
php artisan octane:frankenphp
```

La commande `octane:frankenphp` peut prendre les options suivantes :

* `--host` : L'adresse IP à laquelle le serveur doit se lier (par défaut : `127.0.0.1`)
* `--port` : Le port sur lequel le serveur doit être disponible (par défaut : `8000`)
* `--admin-port` : Le port sur lequel le serveur administratif doit être disponible (par défaut : `2019`)
* `--workers` : Le nombre de workers qui doivent être disponibles pour traiter les requêtes (par défaut : `auto`)
* `--max-requests` : Le nombre de requêtes à traiter avant de recharger le serveur (par défaut : `500`)
* `--caddyfile` : Le chemin vers le fichier `Caddyfile` de FrankenPHP
* `--https` : Activer HTTPS, HTTP/2, et HTTP/3, et générer automatiquement et renouveler les certificats
* `--http-redirect` : Activer la redirection HTTP vers HTTPS (uniquement activé si --https est passé)
* `--watch` : Recharger automatiquement le serveur lorsque l'application est modifiée
* `--poll` : Utiliser le sondage du système de fichiers pendant la surveillance pour surveiller les fichiers sur un réseau
* `--log-level` : Enregistrer les messages au niveau de journalisation spécifié ou au-dessus, en utilisant le logger natif de Caddy

> [!TIP]
> Pour obtenir des logs structurés en JSON logs (utile quand vous utilisez des solutions d'analyse de logs), passez explicitement l'otpion `--log-level`.

En savoir plus sur Laravel Octane [dans sa documentation officielle](https://laravel.com/docs/octane).
