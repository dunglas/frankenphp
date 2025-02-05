<?php

$numberOfRequests = 0;
$printNumberOfRequests = function () use (&$numberOfRequests) {
    $numberOfRequests++;
    echo "requests:$numberOfRequests";
    usleep(10 * 1000);
};

while (frankenphp_handle_request($printNumberOfRequests)) {

}
