<?php

require_once __DIR__.'/_executor.php';

return function () {
    printf(
        'Request body size: %d (%s)',
        strlen(file_get_contents('php://input')),
        $_GET['i'] ?? 'unknown',
    );
};
