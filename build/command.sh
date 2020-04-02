#!/usr/bin/env bash

[ -n "$COMMAND_HELPER" ] && return || readonly COMMAND_HELPER=1
LOCALPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

set -ex

if [ -z "$1" ]; then
    exec $LOCALPATH/fetch-linux-headers.sh
fi

case "$1" in
    help)
	2>&1 echo "Help:
commands:
  sleep60  -- sleep for 60 seconds
  sleep300 -- sleep for 5 minutes
  sleep1800 -- sleep for 1800 seconds
  otherwise try to run whatever was provided
" ;;
    sleep60)  exec sleep 60;;
    sleep300)  exec sleep 300;;
    sleep1800)  exec sleep 1800;;
    *)
	exec $*
	      
	;;
esac

echo "Goodbye"
