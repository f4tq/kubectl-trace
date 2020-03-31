#!/usr/bin/env bash

_term() {
   kill -TERM "${child}" 2>/dev/null
   echo "See ya!"
}

_setup() {
# do something to the
	echo "setup"
}

_start() {

    # skopos watchdog
    #systemctl enable skopos-unleashed-watchdog.timer
	echo "start"
}

_stop() {

    #systemctl stop skopos-unleashed.service
	echo "stop"
}

_install() {
    # explode tar
    tail -n +$[ `grep -n '^TARBALL' $0 | cut -d ':' -f 1` + 1 ] $0 | base64 ${SWITCH} | tar -xzvf -
    _setup
    _start

    # cleanup
    # remove dir instead of rm -rf to make sure everything we packaged is moved to the appropriate place


}

#_good_health() {
#    [ 200 -eq $(curl --connect-timeout 5 --write-out %{http_code} --silent --output /dev/null http://${API_SERVER_HOST}:${API_SERVER_PORT}/health) ]
#}

trap _term SIGTERM SIGINT

set -e
set -x

# assume the first param is a
if ( grep TARBALL $1 ) ; then
    _install $1
else
    $1 &
fi
child=$!
wait "${child}"

set +x
set +e

exit 0


