<?php

require_once __DIR__.'/../_executor.php';

$_ENV['custom_key'] = 'custom_value';

return function () use (&$rememberedIndex) {
    $custom_key = require __DIR__.'/env-global-import.php';
    echo $custom_key;
};