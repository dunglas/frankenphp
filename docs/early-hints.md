# Early Hints

FrankenPHP natively supports the [103 Early Hints status code]().
Using Early Hints can improve the load of your web pages by 30%.

```php
<?php

header('Link: </style.css>; rel=preload; as=style');
headers_send(103);

// your slow algorithms and SQL queries 🤪

echo <<<'HTML'
<!DOCTYPE html>
<title>Hello FrankenPHP</title>
<link rel="stylesheet" href="style.css">
HTML;
```

Early Hints are supported by the normal and the [worker](worker.md) mode.