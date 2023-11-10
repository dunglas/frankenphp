<?php

header('Content-Type: text/plain');

echo file_get_contents('php://input');
