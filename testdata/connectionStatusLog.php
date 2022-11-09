<?php

ignore_user_abort(true);

require_once __DIR__.'/_executor.php';

return function () {
	if(isset($_GET['finish'])) {
		frankenphp_finish_request();
	}
	echo 'hi';
	if(isset($_GET['flush'])) {
		flush();
	}
	$status = (string) connection_status();
	error_log("request {$_GET['i']}: " . $status);
};
