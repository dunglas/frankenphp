<?php

require_once __DIR__ . '/_executor.php';

return function () use (&$requests) {
    $key = $_GET['key'] ?? 'key';
    if (isset($_GET['remember'])) {
        frankenphp_cache_put($key, $_GET['remember'], 60);
    }
    if (isset($_GET['forget'])) {
        frankenphp_cache_forget($key);
    }
    echo frankenphp_cache_get($key) ?? 'nothing';
};
