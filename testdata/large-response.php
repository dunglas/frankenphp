<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo str_repeat("Hey\n", 1024);
};
