<?php

require_once __DIR__.'/../_executor.php';

return function () use (&$rememberedKey) {
    $key = $_GET['key'];
    $put = $_GET['put'] ?? null;

    if(isset($put)){
        putenv("$key=$put");
    }

    $get = getenv($key);
    $asStr = $get === false ? '' : $get;

    echo "$key=$asStr";
};