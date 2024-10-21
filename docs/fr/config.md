# Configuration

FrankenPHP, Caddy ainsi que les modules Mercure et Vulcain peuvent être configurés en utilisant [les formats pris en charge par Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

Dans [les images Docker](docker.md), le `Caddyfile` est situé dans `/etc/caddy/Caddyfile`.
Le binaire statique cherchera le `Caddyfile` dans le répertoire dans lequel il est démarré.
PHP lui-même peut être configuré [en utilisant un fichier `php.ini`](https://www.php.net/manual/fr/configuration.file.php).

Par défaut, le PHP fourni avec les images Docker et celui inclus dans le binaire statique cherchera un fichier `php.ini` dans le répertoire où FrankenPHP est démarré et dans `/usr/local/etc/php/`. Ils chargeront également tous les fichiers se terminant par `.ini` dans `/usr/local/etc/php/conf.d/`.

Aucun fichier `php.ini` n'est présent par défaut, vous devriez copier un modèle officiel fourni par le projet PHP.
Sur Docker, les modèles sont fournis dans les images :

```dockerfile
FROM dunglas/frankenphp

# Production :
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Ou développement :
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

Si vous n'utilisez pas Docker, copiez l'un des fichiers `php.ini-production` ou `php.ini-development` fournis [dans les sources de PHP](https://github.com/php/php-src/).

## Configuration du Caddyfile

Pour enregistrer l'exécutable de FrankenPHP, l'[option globale](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp` doit être définie, puis les [directives HTTP](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` ou `php` peuvent être utilisées dans les blocs de site pour servir votre application PHP.

Exemple minimal :

```caddyfile
{
 # Activer FrankenPHP
 frankenphp
}

localhost {
 # Activer la compression (optionnel)
 encode zstd br gzip
 # Exécuter les fichiers PHP dans le répertoire courant et servir les assets
 php_server
}
```

En option, le nombre de threads à créer et les [workers](worker.md) à démarrer avec le serveur peuvent être spécifiés sous l'option globale.

```caddyfile
{
 frankenphp {
  num_threads <num_threads> # Définit le nombre de threads PHP à démarrer. Par défaut : 2x le nombre de CPUs disponibles.
  worker {
   file <path> # Définit le chemin vers le script worker.
   num <num> # Définit le nombre de threads PHP à démarrer, par défaut 2x le nombre de CPUs disponibles.
   env <key> <value> # Définit une variable d'environnement supplémentaire avec la valeur donnée. Peut être spécifié plusieurs fois pour régler plusieurs variables d'environnement.
  }
 }
}

# ...
```

Vous pouvez également utiliser la forme courte de l'option worker en une seule ligne :

```caddyfile
{
 frankenphp {
  worker <file> <num>
 }
}

# ...
```

Vous pouvez aussi définir plusieurs workers si vous servez plusieurs applications sur le même serveur :

```caddyfile
{
 frankenphp {
  worker /path/to/app/public/index.php <num>
  worker /path/to/other/public/index.php <num>
 }
}

app.example.com {
 root * /path/to/app/public
 php_server
}

other.example.com {
 root * /path/to/other/public
 php_server
}

# ...
```

L'utilisation de la directive `php_server` est généralement suffisante,
mais si vous avez besoin d'un contrôle total, vous pouvez utiliser la directive `php`, qui permet un plus grand niveau de finesse dans la configuration.

Utiliser la directive `php_server` est équivalent à cette configuration :

```caddyfile
route {
 # Ajoute un slash final pour les requêtes de répertoire
 @canonicalPath {
  file {path}/index.php
  not path */
 }
 redir @canonicalPath {path}/ 308
 # Si le fichier demandé n'existe pas, essayer les fichiers index
 @indexFiles file {
  try_files {path} {path}/index.php index.php
  split_path .php
 }
 rewrite @indexFiles {http.matchers.file.relative}
 # FrankenPHP!
 @phpFiles path *.php
 php @phpFiles
 file_server
}
```

Les directives `php_server` et `php` disposent des options suivantes :

```caddyfile
php_server [<matcher>] {
 root <directory> # Définit le dossier racine du le site. Par défaut : valeur de la directive `root` parente.
 split_path <delim...> # Définit les sous-chaînes pour diviser l'URI en deux parties. La première sous-chaîne correspondante sera utilisée pour séparer le "path info" du chemin. La première partie est suffixée avec la sous-chaîne correspondante et sera considérée comme le nom réel de la ressource (script CGI). La seconde partie sera définie comme PATH_INFO pour utilisation par le script. Par défaut : `.php`
 resolve_root_symlink false # Désactive la résolution du répertoire `root` vers sa valeur réelle en évaluant un lien symbolique, s'il existe (activé par défaut).
 env <key> <value> # Définit une variable d'environnement supplémentaire avec la valeur donnée. Peut être spécifié plusieurs fois pour plusieurs variables d'environnement.
}
```

## Variables d'environnement

Les variables d'environnement suivantes peuvent être utilisées pour insérer des directives Caddy dans le `Caddyfile` sans le modifier :

* `SERVER_NAME` : change [les adresses sur lesquelles écouter](https://caddyserver.com/docs/caddyfile/concepts#addresses), les noms d'hôte fournis seront également utilisés pour le certificat TLS généré
* `CADDY_GLOBAL_OPTIONS` : injecte [des options globales](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG` : insère la configuration sous la directive `frankenphp`

Comme pour les SAPI FPM et CLI, les variables d'environnement ne sont exposées par défaut dans la superglobale `$_SERVER`.

La valeur `S` de [la directive `variables_order` de PHP](https://www.php.net/manual/fr/ini.core.php#ini.variables-order) est toujours équivalente à `ES`, que `E` soit défini ailleurs dans cette directive ou non.

## Configuration PHP

Pour charger [des fichiers de configuration PHP supplémentaires](https://www.php.net/manual/fr/configuration.file.php#configuration.file.scan), la variable d'environnement `PHP_INI_SCAN_DIR` peut être utilisée.
Lorsqu'elle est définie, PHP chargera tous les fichiers avec l'extension `.ini` présents dans les répertoires donnés.

## Activer le mode debug

Lors de l'utilisation de l'image Docker, définissez la variable d'environnement `CADDY_GLOBAL_OPTIONS` sur `debug` pour activer le mode debug :

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
