#!/bin/bash

getent group frankenphp &> /dev/null || \
groupadd -r frankenphp &> /dev/null
getent passwd frankenphp &> /dev/null || \
useradd -r -g frankenphp -d /var/lib/frankenphp -s /sbin/nologin -c 'FrankenPHP web server' frankenphp &> /dev/null
exit 0
