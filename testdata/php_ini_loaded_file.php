<?php

require_once __DIR__ . '/_executor.php';

return function () {
    $ini = php_ini_loaded_file();
    if ($ini === false) {
        echo 'none';
    } elseif (str_contains($ini, "testdata/php.ini")) {
        echo 'cwd';
    } else {
        echo 'global';
    }
};