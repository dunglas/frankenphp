<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'This is output '.($_GET['i'] ?? '')."\n";

    frankenphp_finish_request();

    echo 'This is not';
};
