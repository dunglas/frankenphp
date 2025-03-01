<?php

require_once __DIR__ . '/../_executor.php';

return function () use (&$rememberedKey) {
    $keys = $_GET['keys'];

    // echoes ENV1=value1,ENV2=value2
    echo join(',', array_map(fn($key) => "$key=" . $_ENV[$key], $keys));
};
