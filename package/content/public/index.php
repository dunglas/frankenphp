<?php

$i = 0;
$duration = 0;
do {
    $ok = frankenphp_handle_request(function () use ($i, &$duration): void {
        $startTime = microtime(false);
        echo include __DIR__ . '/../example.php';
        $endTime = microtime(false);
        $duration = (float)$endTime - (float)$startTime;
    });

    $i++;
} while ($ok);
