<?php

require_once __DIR__.'/_executor.php';

return function () {
    header('Foo: bar');
    header('Foo2: bar2');
    header('Invalid');
    header('I: ' . ($_GET['i'] ?? 'i not set'));
    http_response_code(201);
    
    echo 'Hello';
};
