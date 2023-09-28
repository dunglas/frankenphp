<?php
require_once __DIR__.'/_executor.php';

return function() {
    $fiber = new Fiber(function() {
        Fiber::suspend('Fiber '.($_GET['i'] ?? ''));
    });
    echo $fiber->start();

    $fiber->resume();
};

