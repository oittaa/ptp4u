# ptp4u
Scalable PTPv2.1 two-step unicast server implementation

Forked from: https://github.com/facebook/time/tree/main/ptp/ptp4u

## Run
Default arguments are good for most of the cases.
However, there is a lot of room for customisation:
```
/usr/local/bin/ptp4u -iface eth1 -workers 20 -monitoringport 1234
```
This will run ptp4u on eth1 with 20 send workers. Instance can be monitored on port 1234.

## Monitoring
By default ptp4u runs http server serving json monitoring data on port 8888. Ex:
```
$ curl -s localhost:8888 | jq
{
  "clockaccuracy": 33,
  "clockclass": 6,
  "drain": 0,
  "reload": 0,
  "rx.delay_req": 60,
  "subscriptions.delay_req": 1,
  "tx.announce": 60,
  "tx.sync": 60,
  "txts.missing": 0,
  "utcoffset_sec": 37,
  "worker.79.subscriptions": 1
}
```
This returns many useful metrics such as the number of active subscriptions, tx/rx stats etc.

## Configuration

```
Usage of ptp4u:
  -config string
        Path to a config with dynamic settings
  -domainnumber uint
        Set the PTP domain by its number. Valid values are [0-255]
  -drainfile string
        ptp4u drain file location (default "/var/tmp/kill_ptp4u")
  -dscp int
        DSCP for PTP packets, valid values are between 0-63 (used by send workers)
  -iface string
        Set the interface (default "eth0")
  -ip string
        IP to bind on (default "::")
  -loglevel string
        Set a log level. Can be: debug, info, warn, error (default "warn")
  -monitoringport int
        Port to run monitoring server on (default 8888)
  -pidfile string
        Pid file location (default "/var/run/ptp4u.pid")
  -pprofaddr string
        host:port for the pprof to bind
  -queue int
        Size of the queue to send out packets
  -recvworkers int
        Set the number of receive workers (default 10)
  -timestamptype value
        Timestamp type. Can be: hardware, software (default hardware)
  -undrainfile string
        ptp4u force undrain file location (default "/var/tmp/unkill_ptp4u")
  -workers int
        Set the number of send workers (default 100)
```

Example dynamic config. It can be reloaded by sending a SIGHUP to the process.
```yml
utcoffset: 37s
clockaccuracy: 33
clockclass: 6
draininterval: 30s
minsubinterval: 1s
maxsubduration: 1h
metricinterval: 1m
```
