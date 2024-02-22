# Configuration

FrankenPHP, Caddy ainsi que les modules Mercure et Vulcain peuvent être configurés en utilisant [les formats pris en charge par Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

Dans l'image Docker, le `Caddyfile` se trouve dans `/etc/caddy/Caddyfile`.

Vous pouvez également configurer PHP en utilisant `php.ini` comme d'habitude.

Dans l'image Docker, le fichier `php.ini` n'est pas présent, vous pouvez le créer ou le `COPY` manuellement :


```dockerfile
FROM dunglas/frankenphp

# Développement :
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini

# Ou production :
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
```

## Configuration du Caddyfile

Pour enregistrer l'exécutable de FrankenPHP, l'[option globale](https://caddyserver.com/docs/caddyfile/concepts#global-options) `frankenphp` doit être définie, puis les [directives HTTP](https://caddyserver.com/docs/caddyfile/concepts#directives) `php_server` ou `php` peuvent être utilisées dans les blocs du site pour servir votre application PHP.

Exemple minimal :

```caddyfile
{
	# Autoriser FrankenPHP
	frankenphp
	# Configurer l'ordre d'exécution de la directive
	order php_server before file_server
}

localhost {
	# Autoriser la compression (optionnel)
	encode zstd br gzip
	# Exécuter les fichiers PHP dans le répertoire courant et servir les ressources
	php_server
}
```

En option, le nombre de threads à créer et les  [scripts de travail](worker.md) à démarrer avec le serveur peuvent être spécifiés sous l'option globale.

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Définit le nombre de threads PHP à démarrer. Par défaut : 2x le nombre de CPUs disponibles.
		worker {
			file <path> # Définit le chemin vers le script de travail.
			num <num> # Définit le nombre de threads PHP à démarrer, par défaut 2x le nombre de CPUs disponibles.
			env <key> <value> # Définit une variable d'environnement supplémentaire à la valeur donnée. Peut être spécifié plusieurs fois pour plusieurs variables d'environnement.
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
...
```

L'utilisation de la directive php_server est généralement suffisante,
mais si vous avez besoin d'un contrôle total, vous pouvez utiliser la directive de niveau inférieur php.

Utiliser la directive php_server est équivalent à cette configuration :

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

Les directives php_server et php disposent des options suivantes :

```caddyfile
php_server [<matcher>] {
	root <directory> # Définit le dossier racine pour le site. Par défaut : directive `root`.
	split_path <delim...> # Définit les sous-chaînes pour diviser l'URI en deux parties. La première sous-chaîne correspondante sera utilisée pour séparer le "path info" du chemin. La première partie est suffixée avec la sous-chaîne correspondante et sera considérée comme le nom réel de la ressource (script CGI). La seconde partie sera définie comme PATH_INFO pour que le script CGI l'utilise. Par défaut : `.php`
	resolve_root_symlink false # Désactive la résolution du répertoire `root` vers sa valeur réelle en évaluant un lien symbolique, s'il existe (activé par défaut).
	env <key> <value> # Définit une variable d'environnement supplémentaire à la valeur donnée. Peut être spécifié plusieurs fois pour plusieurs variables d'environnement.
}
```


## Variables d'environnement

Les variables d'environnement suivantes peuvent être utilisées pour insérer des directives Caddy dans le `Caddyfile` sans le modifier :

* `SERVER_NAME` : change [les adresses sur lesquelles écouter](https://caddyserver.com/docs/caddyfile/concepts#addresses), les noms d'hôte fournis seront également utilisés pour le certificat TLS généré
* `CADDY_GLOBAL_OPTIONS` : injecte [des options globales](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG` : insère la configuration sous la directive `frankenphp`

Contrairement aux SAPI FPM et CLI, les variables d'environnement ne sont **pas** exposées par défaut dans les superglobales `$_SERVER` et `$_ENV`.

Pour propager les variables d'environnement vers `$_SERVER` et `$_ENV`, configurez la directive `variables_order` de `php.ini` sur `EGPCS`.

## Configuration PHP

Pour charger [des fichiers de configuration PHP supplémentaires](https://www.php.net/manual/fr/configuration.file.php#configuration.file.scan), la variable d'environnement `PHP_INI_SCAN_DIR` peut être utilisée.
Lorsqu'elle est définie, PHP chargera tous les fichiers avec l'extension `.ini` présents dans les répertoires donnés.

## Activer le mode Debug

Lors de l'utilisation de l'image Docker, définissez la variable d'environnement `CADDY_GLOBAL_OPTIONS` sur `debug` pour activer le mode debug :

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
