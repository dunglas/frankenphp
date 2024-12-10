<?php

while (frankenphp_handle_request(function () {
    echo "Hello from worker 2";
    // Simulate work to force potential race conditions (phpmainthread_test.go)
    usleep(1000);
})) {

}
