# Early Hints

FrankenPHP prend nativement en charge le code de statut [103 Early Hints](https://developer.chrome.com/blog/early-hints/).
L'utilisation des Early Hints peut améliorer le temps de chargement de vos pages web de 30 %.

```php
<?php

header('Link: </style.css>; rel=preload; as=style');
headers_send(103);

// vos algorithmes lents et requêtes SQL 🤪

echo <<<'HTML'
<!DOCTYPE html>
<title>Hello FrankenPHP</title>
<link rel="stylesheet" href="style.css">
HTML;
```

Les Early Hints sont pris en charge à la fois par les modes "standard" et [worker](worker.md).
