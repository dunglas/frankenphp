<?php

require_once __DIR__.'/_executor.php';

throw new Exception('unexpected');

return function () {
    echo 'hello';
    throw new Exception('error');
};
