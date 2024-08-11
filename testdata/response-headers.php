<?php

require_once __DIR__.'/_executor.php';

return function () {
    header('Foo: bar');
    header('Foo2: bar2');
    header('Invalid');
    header('I: ' . ($_GET['i'] ?? 'i not set'));
    if ($_GET['i'] % 3) {
        http_response_code($_GET['i'] + 100);
    }

    var_export(apache_response_headers());
};
