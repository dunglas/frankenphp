<?php

require_once __DIR__.'/_executor.php';

return function () {
    header('Content-Type: application/json');
    header('HTTP/1.1 204 No Content', true, 204);

    echo '{"status": "test"}';
    flush();
};