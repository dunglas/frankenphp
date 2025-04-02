# Utilisation du mode classique

Sans aucune configuration additionnelle, FrankenPHP fonctionne en mode classique. Dans ce mode, FrankenPHP fonctionne comme un serveur PHP traditionnel, en servant directement les fichiers PHP. Cela en fait un remplaçant parfait à PHP-FPM ou Apache avec mod_php.

Comme Caddy, FrankenPHP accepte un nombre illimité de connexions et utilise un [nombre fixe de threads](config.md#configuration-du-caddyfile) pour les servir. Le nombre de connexions acceptées et en attente n'est limité que par les ressources système disponibles. Le pool de threads PHP fonctionne avec un nombre fixe de threads initialisés au démarrage, comparable au mode statique de PHP-FPM.

Les connexions en file d'attente attendront indéfiniment jusqu'à ce qu'un thread PHP soit disponible pour les servir. Pour éviter cela, définissez un [timeout d'écriture dans Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts) raisonnable.

Chaque instance de Caddy n'utilisera qu'un seul pool de threads FrankenPHP, qui sera partagé par tous les blocs `php_server`.
