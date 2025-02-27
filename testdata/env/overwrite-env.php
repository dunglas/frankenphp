<?php

require_once __DIR__.'/../_executor.php';

// modify $_ENV in the global symbol table
// the modification should persist through the worker's lifetime
$_ENV['custom_key'] = 'custom_value';

return function () use (&$rememberedIndex) {
    $custom_key = require __DIR__.'/import-env.php';
    echo $custom_key;
};
