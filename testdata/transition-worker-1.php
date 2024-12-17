<?php

while (frankenphp_handle_request(function () {
    echo "Hello from worker 1";
})) {

}
