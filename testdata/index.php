<?php

require_once __DIR__.'/_executor.php';

return function () {
    echo sprintf("I am by birth a Genevese (%s)", $_GET['i'] ?? 'i not set');
};
