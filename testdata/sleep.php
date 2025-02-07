<?php

require_once __DIR__ . '/_executor.php';

return function () {
    $sleep = (int)($_GET['sleep'] ?? 0);
    $work = (int)($_GET['work'] ?? 0);
    $output = (int)($_GET['output'] ?? 1);
    $iterations = (int)($_GET['iterations'] ?? 1);

    for ($i = 0; $i < $iterations; $i++) {
        // simulate work
        // with 30_000 iterations we're in the range of a simple Laravel request
        // (without JIT and with debug symbols enabled)
        for ($j = 0; $j < $work; $j++) {
            $a = +$j;
        }

        // simulate IO, sleep x milliseconds
        if ($sleep > 0) {
            usleep($sleep * 1000);
        }

        // simulate output
        for ($k = 0; $k < $output; $k++) {
            echo "slept for $sleep ms and worked for $work iterations";
        }
    }
};
