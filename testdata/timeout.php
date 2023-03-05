<?php

require_once __DIR__.'/_executor.php';

return function () {
    printf("request: %d\n", $_GET['i'] ?? 'unknown');
    set_time_limit(1);

    $x = true;
    $y = 0;
    while ($x) {
        $y++;
    }
};
