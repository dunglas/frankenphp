<?php

require_once __DIR__.'/_executor.php';

return function () {
    var_export($_GET);
    var_export($_POST);
    var_export($_SERVER);
};
