#!/bin/sh

CAASP_INIT_EXE=/usr/local/bin/caasp-init
CAASP_INIT_CONF="/etc/caasp/caasp-init.yaml"
MASTER_CONF="/etc/caasp/master-config.yaml"

ARGS="--v=5"

#####################################################################

PREFIX="[caasp-init-entrypoint]"

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
        if [ -f $CAASP_INIT_CONF ] ; then
            log "Using caasp-init config from $CAASP_INIT_CONF"
            dump $CAASP_INIT_CONF
            ARGS="$ARGS --config=$CAASP_INIT_CONF"
        fi

        if [ -f $MASTER_CONF ] ; then
            log "Using master configuration at $MASTER_CONF"
            dump $MASTER_CONF
            ARGS="$ARGS --kubeadm-config=$MASTER_CONF"
        fi

        chk $CAASP_INIT_EXE
        log "(running: bootstrap $ARGS $@)"
        exec $CAASP_INIT_EXE bootstrap $ARGS $@
        bye 0
        ;;

    reset)
        log "Resetting configuration..."
        chk $CAASP_INIT_EXE
        log "(running: reset $ARGS $@)"
        exec $CAASP_INIT_EXE reset $ARGS $@
        ;;
esac

