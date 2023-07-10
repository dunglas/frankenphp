<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'He';

    ob_flush();
    flush();

    echo 'llo '.($_GET['i'] ?? '');
};
