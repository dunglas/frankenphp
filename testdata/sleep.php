<?php

require_once __DIR__ . '/_executor.php';

return function () {
    $sleep = $_GET['sleep'] ?? 0;
    $work = $_GET['work'] ?? 0;
    $output = $_GET['output'] ?? 1;

    // simulate work
    // 50_000 iterations are approximately the weight of a simple Laravel request
    for ($i = 0; $i < $work; $i++) {
        $a = +$i;
    }

    // simulate IO, some examples:
    // SSDs: 0.1ms - 1ms
    // HDDs: 5ms - 10ms
    // modern databases: usually 1ms - 10ms (for simple queries)
    // external APIs: can take up to multiple seconds
    if ($sleep > 0) {
        usleep($sleep * 1000);
    }

    // simulate output
    for ($i = 0; $i < $output; $i++) {
        echo "slept for $sleep ms and worked for $work iterations";
    }
};
