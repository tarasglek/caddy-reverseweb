#!/bin/bash

# Diagnostic to /tmp
echo "$(date) - CGI script started" >> /tmp/cgi_debug.log

printf "Content-type: text/plain\n"
printf "X-CGI-Debug: true\n\n"

printf "Hello, World!\n"
printf "PATH_INFO    [%s]\n" "$PATH_INFO"
printf "QUERY_STRING [%s]\n" "$QUERY_STRING"

exit 0
