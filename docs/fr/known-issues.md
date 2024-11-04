# Problèmes Connus

## Fibres

Appeller de fonctions et mots clefs PHP qui eux-mêmes appellent [cgo](https://go.dev/blog/cgo) dans des [Fibres](https://www.php.net/manual/fr/language.fibers.php) est connu pour provoquer des plantages.

Ce problème est [en cours de correction par le projet Go](https://github.com/golang/go/issues/62130).

En attendant, une solution consiste à ne pas utiliser de mots clefs (comme `echo`) et de fonctions (comme `header()`) qui délèguent à Go depuis l'intérieur de fibres.

Ce code risque de planter car il utilise `echo` dans une fibre :

```php
$fiber = new Fiber(function() {
    echo 'Dans la fibre'.PHP_EOL;
    echo 'Toujours dedans'.PHP_EOL;
});
$fiber->start();
```

A la place, retournez la valeur de la Fibre et utilisez-la à l'extérieur :

```php
$fiber = new Fiber(function() {
    Fiber::suspend('Dans la fibre'.PHP_EOL));
    Fiber::suspend('Toujours dedans'.PHP_EOL));
});
echo $fiber->start();
echo $fiber->resume();
$fiber->resume();
```

## Extensions PHP non prises en charge

Les extensions suivantes sont connues pour ne pas être compatibles avec FrankenPHP :

| Nom                                                                                                         | Raison          | Alternatives                                                                                                         |
|-------------------------------------------------------------------------------------------------------------|-----------------|----------------------------------------------------------------------------------------------------------------------|
| [imap](https://www.php.net/manual/en/imap.installation.php)                                                 | Non thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |
| [newrelic](https://docs.newrelic.com/docs/apm/agents/php-agent/getting-started/introduction-new-relic-php/) | Non thread-safe | -                                                                                                                    |

## Extensions PHP boguées

Les extensions suivantes ont des bugs connus ou des comportements inattendus lorsqu'elles sont utilisées avec FrankenPHP :

| Nom                                                           | Problème                                                                                                                                                                                                                                                                                                                                      |
|---------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [ext-openssl](https://www.php.net/manual/fr/book.openssl.php) | Lors de l'utilisation d'une version statique de FrankenPHP (construite avec la libc musl), l'extension OpenSSL peut planter sous de fortes charges. Une solution consiste à utiliser une version liée dynamiquement (comme celle utilisée dans les images Docker). Ce bogue est [suivi par PHP](https://github.com/php/php-src/issues/13648). |

## get_browser

La fonction [get_browser()](https://www.php.net/manual/fr/function.get-browser.php) semble avoir de mauvaises performances après un certain temps. Une solution est de mettre en cache (par exemple, avec [APCu](https://www.php.net/manual/en/book.apcu.php)) les résultats par agent utilisateur, car ils sont statiques.

## Binaire autonome et images Docker basées sur Alpine

Le binaire autonome et les images docker basées sur Alpine (`dunglas/frankenphp:*-alpine`) utilisent [musl libc](https://musl.libc.org/) au lieu de [glibc et ses amis](https://www.etalabs.net/compare_libcs.html), pour garder une taille de binaire plus petite. Cela peut entraîner des problèmes de compatibilité. En particulier, le drapeau glob `GLOB_BRACE` n'est [pas disponible](https://www.php.net/manual/fr/function.glob.php).

## Utilisation de `https://127.0.0.1` avec Docker

Par défaut, FrankenPHP génère un certificat TLS pour `localhost`.
C'est l'option est la plus simple et est recommandée pour le développement local.

Si vous voulez vraiment utiliser `127.0.0.1` comme hôte, il est possible de configure FrankenPHP pour générer un certificat pour cela en définissant le nom du serveur à `127.0.0.1`.

Malheureusement, cela ne suffit pas lors de l'utilisation de Docker à cause de [son système de gestion des réseaux](https://docs.docker.com/network/).
Vous obtiendrez une erreur TLS similaire à `curl: (35) LibreSSL/3.3.6: error:1404B438:SSL routines:ST_CONNECT:tlsv1 alert internal error`.

Si vous utilisez Linux, une solution est d'utiliser [le pilote de réseau "hôte"](https://docs.docker.com/network/network-tutorial-host/) :

```console
docker run \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    --network host \
    dunglas/frankenphp
```

Le pilote de réseau "hôte" n'est pas pris en charge sur Mac et Windows. Sur ces plateformes, vous devrez deviner l'adresse IP du conteneur et l'inclure dans les noms de serveur.

Exécutez la commande `docker network inspect bridge` et inpectez la clef `Containers` pour identifier la dernière adresse IP attribuée sous la clef `IPv4Address`, puis incrémentez-la de un. Si aucun conteneur n'est en cours d'exécution, la première adresse IP attribuée est généralement `172.17.0.2`.

Ensuite, incluez ceci dans la variable d'environnement `SERVER_NAME` :

```console
docker run \
    -e SERVER_NAME="127.0.0.1, 172.17.0.3" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

> [!CAUTION]
>
> Assurez-vous de remplacer `172.17.0.3` par l'IP qui sera attribuée à votre conteneur.

Vous devriez maintenant pouvoir accéder à `https://127.0.0.1` depuis la machine hôte.

Si ce n'est pas le cas, lancez FrankenPHP en mode debug pour essayer de comprendre le problème :

```console
docker run \
    -e CADDY_GLOBAL_OPTIONS="debug" \
    -e SERVER_NAME="127.0.0.1" \
    -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
