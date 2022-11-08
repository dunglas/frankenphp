<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'He';

    flush();

    echo 'llo '.($_GET['i'] ?? '');
};
