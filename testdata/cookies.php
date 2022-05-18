<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo var_export($_COOKIE);
};
