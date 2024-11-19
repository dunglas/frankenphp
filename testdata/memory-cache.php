<?php

require_once __DIR__.'/_executor.php';

return function () {
    if (isset($_GET['remember'])) {
        frankenphp_cache_put('remember', $_GET['remember']);
    }
    if (isset($_GET['forget'])) {
        frankenphp_cache_forget('remember');
    }
    echo frankenphp_cache_get('remember') ?? 'nothing';
};
