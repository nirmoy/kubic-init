#!/bin/sh

KUBIC_INIT_EXE=/usr/local/bin/kubic-init
KUBIC_INIT_CONF="/etc/kubic/kubic-init.yaml"

ARGS="--v=5"

#####################################################################

PREFIX="[kubic-init-entrypoint]"

now()    { date +'%Y-%m-%d %H:%M:%S' ; }
bye()    { sleep 1000000 ; exit $1 ; } # TODO: remove this sleep: it is here just for debugging
log()    { echo "# $(now) $PREFIX [INFO] $@" ; }
log_hl() { log ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>" ; }
dump()   { log "Contents of $1:" ; log_hl ; p="# $(now) $PREFIX [INFO] " ; sed -e "s/^/$p/" $1 ; log_hl ; }
warn()   { echo "# $(now) $PREFIX [WARN] $@" ; }
abort()  { echo "# $(now) $PREFIX [ERROR] $@" ; bye 1 ; }
chk()    { [ -x $1 ] || abort "no binary found at $1" ; }

#####################################################################

CMD="bootstrap"
case $1 in
    reset)
        CMD="reset"
        shift
        ;;
    bootstrap)
        shift
        ;;
esac

case $CMD in
    bootstrap)
        log "Bootstrapping node..."
        if [ -f $KUBIC_INIT_CONF ] ; then
            log "Using kubic-init config from $KUBIC_INIT_CONF"
            dump $KUBIC_INIT_CONF
            ARGS="$ARGS --config=$KUBIC_INIT_CONF"
        fi

        chk $KUBIC_INIT_EXE
        log "(running: bootstrap $ARGS $@)"
        exec $KUBIC_INIT_EXE bootstrap $ARGS $@
        bye 0
        ;;

    reset)
        log "Resetting configuration..."
        chk $KUBIC_INIT_EXE
        log "(running: reset $ARGS $@)"
        exec $KUBIC_INIT_EXE reset $ARGS $@
        ;;
esac

