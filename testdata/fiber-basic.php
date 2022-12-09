<?php
require_once __DIR__.'/_executor.php';

return function(){
    $fiber = new Fiber(function(){
        echo "OK";
    });
    $fiber->start();
};

