<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo 'hello';
    throw new Exception("request {$_GET['i']}");
};
