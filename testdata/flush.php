<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'He';

    flush();
    sleep(2);

    echo 'llo';
};
