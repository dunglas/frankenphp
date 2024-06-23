<?php

$workerServer = $_SERVER;

require_once __DIR__.'/_executor.php';

return function () use ($workerServer) {
    echo $_SERVER['FOO'] ?? '';
    echo $workerServer['FOO'] ?? '';
    echo $_GET['i'] ?? '';
};
