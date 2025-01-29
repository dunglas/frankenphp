<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo $_GET['key'] . ':' . ini_get($_GET['key']);
};
