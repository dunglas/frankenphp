<?php

apache_request_headers();

require_once __DIR__.'/_executor.php';

return function() {
    print_r(apache_request_headers());
};
