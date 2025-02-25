<?php

require_once __DIR__.'/../_executor.php';

$rememberedIndex = 0;

return function () use (&$rememberedIndex) {
    $indexFromRequest = $_GET['index'] ?? null;
    if(isset($indexFromRequest)){
        $rememberedIndex = (int)$indexFromRequest;
        putenv("index=$rememberedIndex");
    }

    $indexInEnv = (int)getenv('index');
    if($indexInEnv === $rememberedIndex){
        echo 'success';
        return;
    }
    echo "failure: '$indexInEnv' is not '$rememberedIndex'";
};