<?php

require_once __DIR__.'/_executor.php';
require_once __DIR__.'/autoloader-require.php';

return function () {
    echo "request {$_GET['i']}\n";
    echo implode(',', spl_autoload_functions());
};
