#!/bin/bash

if [ "$1" -eq 1 ] && [ -x "/usr/lib/systemd/systemd-update-helper" ]; then
    # Initial installation
    /usr/lib/systemd/systemd-update-helper install-system-units frankenphp.service || :
fi

if [ -x /usr/sbin/getsebool ]; then
    # connect to ACME endpoint to request certificates
    setsebool -P httpd_can_network_connect on
fi
if [ -x /usr/sbin/semanage ] && [ -x /usr/sbin/restorecon ]; then
    # file contexts
    semanage fcontext --add --type httpd_exec_t        '/usr/bin/frankenphp'         2> /dev/null || :
    semanage fcontext --add --type httpd_sys_content_t '/usr/share/frankenphp(/.*)?' 2> /dev/null || :
    semanage fcontext --add --type httpd_config_t      '/etc/frankenphp(/.*)?'       2> /dev/null || :
    semanage fcontext --add --type httpd_var_lib_t     '/var/lib/frankenphp(/.*)?'   2> /dev/null || :
    restorecon -r /usr/bin/frankenphp /usr/share/frankenphp /etc/frankenphp /var/lib/frankenphp || :
fi
if [ -x /usr/sbin/semanage ]; then
    # QUIC
    semanage port --add --type http_port_t --proto udp 80   2> /dev/null || :
    semanage port --add --type http_port_t --proto udp 443  2> /dev/null || :
    # admin endpoint
    semanage port --add --type http_port_t --proto tcp 2019 2> /dev/null || :
fi
