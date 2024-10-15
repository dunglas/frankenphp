# Performance

Par défaut, FrankenPHP essaie d'offrir un bon compromis entre performance et facilité d'utilisation.
Cependant, il est possible d'améliorer considérablement les performances en utilisant une configuration appropriée.

## Nombre de threads et de workers

Par défaut, FrankenPHP démarre 2 fois plus de threads et de workers (en mode worker) que le nombre de CPU disponibles.

Les valeurs appropriées dépendent fortement de la manière dont votre application est écrite, de ce qu'elle fait et de votre matériel.
Nous recommandons vivement de modifier ces valeurs.

Pour trouver les bonnes valeurs, il est souhaitable d'effectuer des tests de charge simulant le trafic réel.
[k6](https://k6.io) et [Gatling](https://gatling.io) sont de bons outils pour cela.

Pour configurer le nombre de threads, utilisez l'option `num_threads` des directives `php_server` et `php`.
Pour changer le nombre de travailleurs, utilisez l'option `num` de la section `worker` de la directive `frankenphp`.

## Mode worker

Activer [le mode worker](worker.md) améliore considérablement les performances,
mais votre application doit être adaptée pour être compatible avec ce mode :
vous devez créer un script worker et vous assurer que l'application n'a pas de fuite de mémoire.

## Ne pas utiliser musl

Les binaires statiques que nous fournissons, ainsi que la variante Alpine Linux des images Docker officielles, utilisent [la bibliothèque musl](https://musl.libc.org).

PHP est connu pour être significativement plus lent lorsqu'il utilise cette bibliothèque C alternative au lieu de la bibliothèque GNU traditionnelle,
surtout lorsqu'il est compilé en mode ZTS (thread-safe), ce qui est nécessaire pour FrankenPHP.

En outre, certains bogues ne se produisent que lors de l'utilisation de musl.

Dans les environnements de production, nous recommandons fortement d'utiliser la glibc.

Cela peut être réalisé en utilisant les images Docker Debian (par défaut) et [en compilant FrankenPHP à partir des sources](compile.md).

Alternativement, nous fournissons des binaires statiques compilés avec [l'allocateur mimalloc](https://github.com/microsoft/mimalloc), ce qui rend FrankenPHP+musl plus rapide (mais toujours plus lent que FrankenPHP+glibc).

## Configuration du runtime Go

FrankenPHP est écrit en Go.

En général, le runtime Go ne nécessite pas de configuration particulière, mais dans certaines circonstances,
une configuration spécifique améliore les performances.

Vous voudrez probablement mettre la variable d'environnement `GODEBUG` à `cgocheck=0` (la valeur par défaut dans les images Docker de FrankenPHP).

Si vous exécutez FrankenPHP dans des conteneurs (Docker, Kubernetes, LXC...) et que vous limitez la mémoire disponible pour les conteneurs,
mettez la variable d'environnement `GOMEMLIMIT` à la quantité de mémoire disponible.

Pour plus de détails, [la page de documentation Go dédiée à ce sujet](https://pkg.go.dev/runtime#hdr-Environment_Variables) est à lire absolument pour tirer le meilleur parti du runtime.

## `file_server`

Par défaut, la directive `php_server` met automatiquement en place un serveur de fichiers pour
pour servir les fichiers statiques (assets) stockés dans le répertoire racine.

Cette fonctionnalité est pratique, mais a un coût.
Pour la désactiver, utilisez la configuration suivante :

```caddyfile
php_server {
    file_server off
}
```

## *Placeholders*

Vous pouvez utiliser des [*placeholders*](https://caddyserver.com/docs/conventions#placeholders) dans les directives `root` et `env`.
Cependant, cela empêche la mise en cache de ces valeurs et a un coût important en termes de performances.

Si possible, évitez les *placeholders* dans ces directives.

## `resolve_root_symlink`

Par défaut, si le *document root* est un lien symbolique, il est automatiquement résolu par FrankenPHP (c'est nécessaire pour le bon fonctionnement de PHP).
Si la racine du document n'est pas un lien symbolique, vous pouvez désactiver cette fonctionnalité.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

Cela améliorera les performances si la directive `root` contient des [*placeholders*](https://caddyserver.com/docs/conventions#placeholders).
Le gain sera négligeable dans les autres cas.

## Journaux

La journalisation est évidemment très utile, mais, par définition, elle nécessite des opérations d'*I/O* et des allocations de mémoire,
ce qui réduit considérablement les performances.
Assurez-vous de [définir le niveau de journalisation](https://caddyserver.com/docs/caddyfile/options#log) correctement,
et de ne journaliser que ce qui est nécessaire.

## Performances de PHP

FrankenPHP utilise l'interpréteur PHP officiel.
Toutes les optimisations de performances habituelles liées à PHP s'appliquent à FrankenPHP.

En particulier :

* vérifiez que [OPcache](https://www.php.net/manual/en/book.opcache.php) est installé, activé et correctement configuré
* activez [les optimisations de l'autoloader de Composer](https://getcomposer.org/doc/articles/autoloader-optimization.md)
* assurez-vous que le cache `realpath` est suffisamment grand pour les besoins de votre application
* utilisez le [pré-chargement](https://www.php.net/manual/en/opcache.preloading.php)

Pour plus de détails, lisez [l'entrée de la documentation dédiée de Symfony](https://symfony.com/doc/current/performance.html)
(la plupart des conseils sont utiles même si vous n'utilisez pas Symfony).
