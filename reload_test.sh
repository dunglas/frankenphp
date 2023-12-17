#!/bin/bash
for ((i = 0 ; i < 100 ; i++)); do
    curl http://localhost:2019/config/apps/frankenphp
    curl -H 'Cache-Control: must-revalidate' --json '{"workers":[{"file_name":"./index.php"}]}' -X PATCH http://localhost:2019/config/apps/frankenphp
done
